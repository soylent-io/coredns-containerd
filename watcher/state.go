package watcher

import (
	"context"

	"github.com/containerd/containerd/api/events"
	"github.com/docker/docker/api/types"
	"github.com/docker/docker/client"
	log "github.com/sirupsen/logrus"
)

// State manage containers as a CRUD
type State struct {
	watcher    *Watcher
	containers map[string]*types.ContainerJSON
}

// NewState returns a new State
func NewState(socketContainerd, socketDocker string) (*State, error) {
	w, err := New(socketContainerd, socketDocker)
	if err != nil {
		return nil, err
	}
	s := &State{
		watcher:    w,
		containers: make(map[string]*types.ContainerJSON),
	}
	err = s.watcher.HandleStart("", func(cont *types.ContainerJSON, event *events.TaskStart) {
		s.containers[cont.ID] = cont
	})
	if err != nil {
		return nil, err
	}
	err = s.watcher.HandleExit("", func(cont *types.ContainerJSON, event *events.TaskExit) {
		delete(s.containers, cont.ID)
	})
	if err != nil {
		return nil, err
	}
	return s, nil
}

// HandleCreate handles create events
func (s *State) HandleCreate(filter string, handler func(*types.ContainerJSON)) error {
	return s.watcher.HandleStart(filter, func(cont *types.ContainerJSON, event *events.TaskStart) {
		handler(cont)
	})
}

// HandleDelete handles delete events
func (s *State) HandleDelete(filter string, handler func(*types.ContainerJSON)) error {
	return s.watcher.HandleExit(filter, func(cont *types.ContainerJSON, event *events.TaskExit) {
		handler(cont)
	})
}

// Listen looks for already existing containers and watch for events
func (s *State) Listen(ctxw context.Context) error {
	ctx := context.Background()
	containers, err := s.watcher.Container.Containers(ctx, "")
	if err != nil {
		return err
	}
	for _, container := range containers {
		go func(docker *client.Client, id string) {
			ctx := context.Background()
			cont, err := docker.ContainerInspect(ctx, id)
			if err != nil {
				log.Error(err)
				return
			}
			for _, h := range s.watcher.startHandlers {
				h.handler(&cont, nil)
			}
		}(s.watcher.Docker, container.ID())
	}
	s.watcher.Listen(ctxw)
	return nil
}
