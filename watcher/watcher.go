package watcher

import (
	"context"
	"fmt"
	"os"

	log "github.com/sirupsen/logrus"

	"github.com/containerd/containerd"
	_events "github.com/containerd/containerd/api/events"
	"github.com/containerd/containerd/events"
	"github.com/containerd/containerd/filters"
	"github.com/containerd/containerd/runtime"
	"github.com/containerd/typeurl/v2"
)

// Watcher watch the watchmen
// Watch Containerd for container events
type Watcher struct {
	Container      *containerd.Client
	startHandlers  []*startHandler
	exitHandlers   []*exitHandler
	deleteHandlers []*deleteHandler
}

type startHandler struct {
	filter  filters.Filter
	handler func(containerd.Container, *_events.TaskStart)
}

type exitHandler struct {
	filter  filters.Filter
	handler func(containerd.Container, *_events.TaskExit)
}

type deleteHandler struct {
	filter  filters.Filter
	handler func(containerd.Container, *_events.TaskDelete)
}

// New Watcher
func New(socketContainerd string) (*Watcher, error) {
	if socketContainerd == "" {
		socketContainerd = os.Getenv("CONTAINERD_SOCKET")
		if socketContainerd == "" {
			// Default Debian value
			socketContainerd = "/var/run/containerd/containerd.sock"
		}
	}
	cli, err := containerd.New(socketContainerd, containerd.WithDefaultNamespace("k8s.io"))
	if err != nil {
		return nil, err
	}

	return &Watcher{
		Container: cli,
	}, nil
}

// Version of containerd
func (w *Watcher) Version() (containerd.Version, error) {
	return w.Container.Version(context.Background())
}

// HandleStart handles start
func (w *Watcher) HandleStart(filter string, handler func(containerd.Container, *_events.TaskStart)) error {
	f, err := filters.Parse(filter)
	if err != nil {
		return err
	}
	w.startHandlers = append(w.startHandlers, &startHandler{
		filter:  f,
		handler: handler,
	})
	return nil
}

// HandleExit handles exit
func (w *Watcher) HandleExit(filter string, handler func(containerd.Container, *_events.TaskExit)) error {
	f, err := filters.Parse(filter)
	if err != nil {
		return err
	}
	w.exitHandlers = append(w.exitHandlers, &exitHandler{
		filter:  f,
		handler: handler,
	})
	return nil
}

// HandleDelete handles delete
func (w *Watcher) HandleDelete(filter string, handler func(containerd.Container, *_events.TaskDelete)) error {
	f, err := filters.Parse(filter)
	if err != nil {
		return err
	}
	w.deleteHandlers = append(w.deleteHandlers, &deleteHandler{
		filter:  f,
		handler: handler,
	})
	return nil
}

// Listen events
func (w *Watcher) Listen(ctxw context.Context) {
	ctx := context.Background()
	ch, errs := w.Container.Subscribe(ctx, "namespace==k8s.io")
	for {
		select {
		case <-ctxw.Done():
			// Exit the loop
			return
		case err := <-errs:
			log.Error(err)
		case c := <-ch:
			go func(c *events.Envelope) {
				v, err := typeurl.UnmarshalAny(c.Event)
				if err != nil {
					log.Error(err)
					return
				}
				switch c.Topic {
				case runtime.TaskStartEventTopic:
					start, ok := v.(*_events.TaskStart)
					if !ok {
						log.Error(fmt.Errorf("can't cast to TaskStart : %s", start))
						return
					}
					ctxCont := context.Background()
					contc, err := w.Container.LoadContainer(ctxCont, start.ContainerID)
					if err != nil {
						log.Error(err)
						return
					}
					ca := NewContainerAdaptor(contc)
					for _, h := range w.startHandlers {
						if h.filter.Match(ca) {
							go h.handler(contc, start)
						}
					}
				case runtime.TaskExitEventTopic:
					exit, ok := v.(*_events.TaskExit)
					if !ok {
						log.Error(fmt.Errorf("can't cast to TaskExit : %s", exit))
						return
					}
					ctxCont := context.Background()
					contc, err := w.Container.LoadContainer(ctxCont, exit.ContainerID)
					if err != nil {
						log.Error(err)
						return
					}
					ca := NewContainerAdaptor(contc)
					for _, h := range w.exitHandlers {
						if h.filter.Match(ca) {
							go h.handler(contc, exit)
						}
					}
				case runtime.TaskDeleteEventTopic:
					delete, ok := v.(*_events.TaskDelete)
					if !ok {
						log.Error(fmt.Errorf("can't cast to TaskDelete: %s", delete))
						return
					}
					ctxCont := context.Background()
					contc, err := w.Container.LoadContainer(ctxCont, delete.ContainerID)
					if err != nil {
						log.Error(err)
						return
					}
					ca := NewContainerAdaptor(contc)
					for _, h := range w.deleteHandlers {
						if h.filter.Match(ca) {
							go h.handler(contc, delete)
						}
					}
				}
			}(c)
		}
	}
}
