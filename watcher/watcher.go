package watcher

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"

	"gopkg.in/yaml.v2"

	"github.com/docker/docker/api/types"

	log "github.com/sirupsen/logrus"

	"github.com/containerd/containerd"
	_events "github.com/containerd/containerd/api/events"
	"github.com/containerd/containerd/events"
	"github.com/containerd/containerd/filters"
	"github.com/containerd/containerd/runtime"
	"github.com/containerd/typeurl"
	"github.com/docker/docker/client"
	"github.com/opencontainers/runtime-spec/specs-go"
)

// Watcher watch the watchmen
// Watch Containerd for Docker events
type Watcher struct {
	Container      *containerd.Client
	Docker         *client.Client
	startHandlers  []*startHandler
	exitHandlers   []*exitHandler
	deleteHandlers []*deleteHandler
}

type startHandler struct {
	filter  filters.Filter
	handler func(*types.ContainerJSON, *_events.TaskStart)
}

type exitHandler struct {
	filter  filters.Filter
	handler func(*types.ContainerJSON, *_events.TaskExit)
}

type deleteHandler struct {
	filter  filters.Filter
	handler func(*types.ContainerJSON, *_events.TaskDelete)
}

// New Watcher
func New(socketContainerd, socketDocker string) (*Watcher, error) {
	if socketContainerd == "" {
		socketContainerd = os.Getenv("CONTAINERD_SOCKET")
		if socketContainerd == "" {
			// Default Debian value
			socketContainerd = "/var/run/containerd/containerd.sock"
		}
	}
	cli, err := containerd.New(socketContainerd, containerd.WithDefaultNamespace("moby"))
	if err != nil {
		return nil, err
	}
	var clientDocker *client.Client
	if socketDocker == "" {
		clientDocker, err = client.NewEnvClient()
	} else {
		clientDocker, err = client.NewClient(socketDocker, "", &http.Client{}, map[string]string{})
	}
	if err != nil {
		return nil, err
	}
	return &Watcher{
		Container: cli,
		Docker:    clientDocker,
	}, nil
}

// Version of containerd
func (w *Watcher) Version() (containerd.Version, error) {
	return w.Container.Version(context.Background())
}

// HandleStart handles start
func (w *Watcher) HandleStart(filter string, handler func(*types.ContainerJSON, *_events.TaskStart)) error {
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
func (w *Watcher) HandleExit(filter string, handler func(*types.ContainerJSON, *_events.TaskExit)) error {
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
func (w *Watcher) HandleDelete(filter string, handler func(*types.ContainerJSON, *_events.TaskDelete)) error {
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
	ch, errs := w.Container.Subscribe(ctx, "namespace==moby")
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
				ctx := context.Background()
				switch c.Topic {
				case runtime.TaskStartEventTopic:
					start, ok := v.(*_events.TaskStart)
					if !ok {
						log.Error(fmt.Errorf("Can't cast to TaskStart : %s", start))
						return
					}
					cont, err := w.Docker.ContainerInspect(ctx, start.ContainerID)
					if err != nil {
						log.Error(err)
						return
					}
					ctxCont := context.Background()
					contc, err := w.Container.LoadContainer(ctxCont, start.ContainerID)
					if err != nil {
						log.Error(err)
						return
					}
					info, err := contc.Info(ctxCont)
					if err != nil {
						log.Error(err)
						return
					}
					if info.Spec.GetTypeUrl() == "types.containerd.io/opencontainers/runtime-spec/1/Spec" {
						spec := specs.Spec{}
						err = json.Unmarshal(info.Spec.Value, &spec)
						if err != nil {
							log.Error(err)
							return
						}
						b, err := yaml.Marshal(spec)
						if err != nil {
							log.Error(err)
							return
						}
						fmt.Println(string(b))
					}
					ca := NewContainerAdaptor(&cont)
					for _, h := range w.startHandlers {
						if h.filter.Match(ca) {
							go h.handler(&cont, start)
						}
					}
				case runtime.TaskExitEventTopic:
					exit, ok := v.(*_events.TaskExit)
					if !ok {
						log.Error(fmt.Errorf("Can't cast to TaskExit : %s", exit))
						return
					}
					cont, err := w.Docker.ContainerInspect(ctx, exit.ContainerID)
					if err != nil {
						log.Error(err)
						return
					}
					ca := NewContainerAdaptor(&cont)
					for _, h := range w.exitHandlers {
						if h.filter.Match(ca) {
							go h.handler(&cont, exit)
						}
					}
				case runtime.TaskDeleteEventTopic:
					delete, ok := v.(*_events.TaskDelete)
					if !ok {
						log.Error(fmt.Errorf("Can't cast to TaskDelete: %s", delete))
						return
					}
					cont, err := w.Docker.ContainerInspect(ctx, delete.ContainerID)
					if err != nil {
						log.Error(err)
						return
					}
					ca := NewContainerAdaptor(&cont)
					for _, h := range w.deleteHandlers {
						if h.filter.Match(ca) {
							go h.handler(&cont, delete)
						}
					}
				}
			}(c)
		}
	}
}
