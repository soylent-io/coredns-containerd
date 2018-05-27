package watcher

import (
	"context"

	log "github.com/sirupsen/logrus"

	"github.com/containerd/containerd"
	"github.com/containerd/typeurl"
	// Register grpc event types
	"github.com/containerd/containerd/api/events"
)

type Watcher struct {
	client *containerd.Client
}

func New(socket string) (*Watcher, error) {
	client, err := containerd.New(socket, containerd.WithDefaultNamespace("moby"))
	if err != nil {
		return nil, err
	}
	return &Watcher{
		client: client,
	}, nil
}

func (w *Watcher) Version() (containerd.Version, error) {
	return w.client.Version(context.Background())
}

func (w *Watcher) Watch() {
	ctx := context.Background()
	ch, errs := w.client.Subscribe(ctx, "namespace==moby")
	for {
		select {
		case c := <-ch:
			v, err := typeurl.UnmarshalAny(c.Event)
			if err != nil {
				log.Error("Unmarshal error: ", err)
			} else {
				log.Printf("%s #%s#\n\t%s\n", c.Namespace, c.Topic, v)
				if c.Namespace == "moby" {
					switch c.Topic {
					case "/tasks/start":
						log.Info(v)
						start, ok := v.(*events.TaskStart)
						if ok {
							cont, err := w.client.ContainerService().Get(ctx, start.ContainerID)
							if err != nil {
								log.Error("getContainer: ", err)
							} else {
								log.Info("start", cont)
							}
						} else {
							log.Error("Can't cast ", start)
						}
					default:
						log.Info("Unknown topic: ", c.Topic)
					}
				} else {
					log.Info("Strange namespace: %s", c.Namespace)
				}
			}
		case e := <-errs:
			log.Info(e)
		}
	}
}
