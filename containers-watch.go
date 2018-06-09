package main

import (
	"os"

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
	w.Watch()
}
