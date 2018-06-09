package watcher

import (
	"context"
	"fmt"
	"net/http"

	"github.com/docker/docker/api/types"
	log "github.com/sirupsen/logrus"

	"github.com/containerd/containerd"
	"github.com/containerd/containerd/api/events"
	eventsapi "github.com/containerd/containerd/api/services/events/v1"
	"github.com/containerd/containerd/filters"
	"github.com/containerd/containerd/runtime"
	"github.com/containerd/typeurl"
	"github.com/docker/docker/client"
	"github.com/opencontainers/runtime-spec/specs-go"
)

// Watcher watch the watchmen
// Watch Containerd for Docker events
type Watcher struct {
	client         *containerd.Client
	docker         *client.Client
	startHandlers  []*startHandler
	exitHandlers   []*exitHandler
	deleteHandlers []*deleteHandler
}

type startHandler struct {
	filter  filters.Filter
	handler func(*types.ContainerJSON, *events.TaskStart)
}

type exitHandler struct {
	filter  filters.Filter
	handler func(*types.ContainerJSON, *events.TaskExit)
}

type deleteHandler struct {
	filter  filters.Filter
	handler func(*types.ContainerJSON, *events.TaskDelete)
}

// New Watcher
func New(socketContainerd, socketDocker string) (*Watcher, error) {
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
		client: cli,
		docker: clientDocker,
	}, nil
}

// Version of containerd
func (w *Watcher) Version() (containerd.Version, error) {
	return w.client.Version(context.Background())
}

// HandleStart handles start
func (w *Watcher) HandleStart(filter string, handler func(*types.ContainerJSON, *events.TaskStart)) error {
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
func (w *Watcher) HandleExit(filter string, handler func(*types.ContainerJSON, *events.TaskExit)) error {
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
func (w *Watcher) HandleDelete(filter string, handler func(*types.ContainerJSON, *events.TaskDelete)) error {
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
	ch, errs := w.client.Subscribe(ctx, "namespace==moby")
	for {
		select {
		case <-ctxw.Done():
			// Exit the loop
			return
		case err := <-errs:
			log.Error(err)
		case c := <-ch:
			go func(c *eventsapi.Envelope) {
				v, err := typeurl.UnmarshalAny(c.Event)
				if err != nil {
					log.Error(err)
					return
				}
				ctx := context.Background()
				switch c.Topic {
				case runtime.TaskStartEventTopic:
					start, ok := v.(*events.TaskStart)
					if !ok {
						log.Error(fmt.Errorf("Can't cast to TaskStart : %s", start))
						return
					}
					cont, err := w.docker.ContainerInspect(ctx, start.ContainerID)
					if err != nil {
						log.Error(err)
						return
					}
					ca := NewContainerAdaptor(&cont)
					for _, h := range w.startHandlers {
						if h.filter.Match(ca) {
							go h.handler(&cont, start)
						}
					}
				case runtime.TaskExitEventTopic:
					exit, ok := v.(*events.TaskExit)
					if !ok {
						log.Error(fmt.Errorf("Can't cast to TaskExit : %s", exit))
						return
					}
					cont, err := w.docker.ContainerInspect(ctx, exit.ContainerID)
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
					delete, ok := v.(*events.TaskDelete)
					if !ok {
						log.Error(fmt.Errorf("Can't cast to TaskDelete: %s", delete))
						return
					}
					cont, err := w.docker.ContainerInspect(ctx, delete.ContainerID)
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

// Watch events
func (w *Watcher) Watch() {
	ctx := context.Background()
	ch, errs := w.client.Subscribe(ctx, "namespace==moby")
	for {
		select {
		case c := <-ch:
			v, err := typeurl.UnmarshalAny(c.Event)
			if err != nil {
				log.Error("Unmarshal event error: ", err)
				break
			}
			log.Printf("%s #%s#\n\t%s\n", c.Namespace, c.Topic, v)
			if c.Namespace != "moby" {
				log.Info("Strange namespace: %s", c.Namespace)
				break
			}
			switch c.Topic {
			case "/tasks/start":
				start, ok := v.(*events.TaskStart)
				if !ok {
					log.Error("Can't cast ", start)
					break
				}
				cont, err := w.client.LoadContainer(ctx, start.ContainerID)

				if err != nil {
					log.Error("getContainer: ", start.ContainerID, " ", err)
					break
				}
				info, err := cont.Info(ctx)
				if err != nil {
					log.Error("Info error: ", start.ContainerID, " ", err)
					break
				}
				log.Info("Info: ", info)
				s, err := typeurl.UnmarshalAny(info.Spec)
				if err != nil {
					log.Error("Can't unmarshal info spec :", start.ContainerID, " ", err)
					break
				}
				spec, ok := s.(*specs.Spec)
				if !ok {
					log.Error("Can't cast ", s)
					break
				}
				log.Info("start spec: ", spec)
				log.Info("start spec annotation: ", spec.Annotations)

				labels, err := cont.Labels(ctx)
				if err != nil {
					log.Error("Labels error: ", err)
					break
				}
				log.Info("Labels: ", labels)

				/*
					log.Info("start spec type: ", info.Spec.GetTypeUrl())
					log.Info("start spec value: ", string(info.Spec.GetValue()))
					log.Info("start annotations: ", spec.Annotations)
				*/
			default:
				log.Info("Unknown topic ", c.Topic)
			}
		case e := <-errs:
			log.Info(e)
		}
	}
}
