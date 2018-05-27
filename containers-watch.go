package main

import (
	log "github.com/sirupsen/logrus"
	"gitlab.bearstech.com/factory/containers-watch/watcher"
)

func main() {
	w, err := watcher.New("/var/run/docker/containerd/docker-containerd.sock")
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
