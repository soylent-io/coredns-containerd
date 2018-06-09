package main

import (
	"context"

	"github.com/containerd/containerd/api/events"
	"github.com/docker/docker/api/types"
	log "github.com/sirupsen/logrus"
	"gitlab.bearstech.com/factory/containers-watch/watcher"
)

func main() {
	w, err := watcher.New("", "")
	if err != nil {
		panic(err)
	}
	v, err := w.Version()
	if err != nil {
		panic(err)
	}
	log.Info(v)
	w.HandleStart("", func(cont *types.ContainerJSON, event *events.TaskStart) {
		log.Info("Start: ", cont, " ", event)
	})
	w.HandleExit("", func(cont *types.ContainerJSON, event *events.TaskExit) {
		log.Info("Exit: ", cont, " ", event)
	})
	ctx := context.Background()
	w.Listen(ctx)
}
