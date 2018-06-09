package main

import (
	"context"
	"os"

	"github.com/containerd/containerd/api/events"
	"github.com/docker/docker/api/types"
	log "github.com/sirupsen/logrus"
	"gitlab.bearstech.com/factory/containers-watch/watcher"
)

func main() {
	socket := os.Getenv("CONTAINERD_SOCKET")
	if socket == "" {
		socket = "/var/run/docker/containerd/docker-containerd.sock"
	}
	w, err := watcher.New(socket, "")
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
