package watcher

import (
	"context"

	log "github.com/sirupsen/logrus"

	"github.com/containerd/containerd"
	"github.com/containerd/containerd/api/events"
	"github.com/containerd/typeurl"
	"github.com/opencontainers/runtime-spec/specs-go"
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
