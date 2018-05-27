package watcher

import (
	"context"

	log "github.com/sirupsen/logrus"

	"github.com/containerd/containerd"
	"github.com/containerd/typeurl"
	// Register grpc event types
	_ "github.com/containerd/containerd/api/events"
)

type Watcher struct {
	client *containerd.Client
}

func New(socket string) (*Watcher, error) {
	client, err := containerd.New(socket)
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
				log.Error(err)
			} else {
				log.Printf("%s %s\n\t%s\n", c.Namespace, c.Topic, v)
				//log.Printf("Topic: %s Namespace: %s\n\tTypeUrl: %s\n\tValue: %s", c.Topic, c.Namespace, c.Event.GetTypeUrl(), string(c.Event.GetValue()))

			}
		case e := <-errs:
			log.Info(e)
		}
	}
}
