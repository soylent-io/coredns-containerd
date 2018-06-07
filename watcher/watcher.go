package watcher

import (
	"context"
	"fmt"
	"net/http"

	"github.com/docker/docker/api/types"
	log "github.com/sirupsen/logrus"

	"github.com/containerd/containerd"
	"github.com/containerd/containerd/api/events"
	"github.com/containerd/typeurl"
	"github.com/docker/docker/client"
	"github.com/opencontainers/runtime-spec/specs-go"
)

// Containers events
const (
	TaskStart = "/task/start" // Container start
)

// Watcher watch the watchmen
// Watch Containerd for Docker events
type Watcher struct {
	client *containerd.Client
	docker *client.Client
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

// Subscribe for containers events
func (w *Watcher) Subscribe(ctx context.Context, topic string, filters ...string) (chan *types.Container, chan error) {
	cc := make(chan *types.Container, 1)
	ce := make(chan error, 1)
	ch, errs := w.client.Subscribe(ctx, "namespace==moby")
	go func() {
		for {
			select {
			case e := <-errs:
				ce <- e
			case c := <-ch:
				if c.Topic != topic {
					break
				}
				v, err := typeurl.UnmarshalAny(c.Event)
				if err != nil {
					ce <- err
					break
				}
				switch c.Topic {
				case TaskStart:
					start, ok := v.(*events.TaskStart)
					if !ok {
						ce <- fmt.Errorf("Can't cast to TaskStart : %s", start)
						break
					}
					cont, err := w.docker.ContainerInspect(ctx, start.ContainerID)
					if err != nil {
						ce <- err
						break
					}
					log.Info(cont)
				}

			}
		}
	}()
	return cc, ce
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
