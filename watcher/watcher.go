package watcher

import (
	"context"

	log "github.com/sirupsen/logrus"

	"github.com/containerd/containerd"
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
	ch, errs := w.client.Subscribe(ctx)
	for {
		select {
		case c := <-ch:
			log.Info(c)
		case e := <-errs:
			log.Info(e)
		}
	}
}
