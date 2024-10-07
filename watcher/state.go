package watcher

import (
	"context"
	"fmt"

	"github.com/containerd/containerd"
	"github.com/containerd/containerd/api/events"
	log "github.com/sirupsen/logrus"
)

// State manage containers as a CRUD
type State struct {
	watcher    *Watcher
	containers map[string]containerd.Container
}

// NewState returns a new State
func NewState(socketContainerd string) (*State, error) {
	w, err := New(socketContainerd)
	if err != nil {
		return nil, err
	}
	s := &State{
		watcher:    w,
		containers: make(map[string]containerd.Container),
	}
	err = s.watcher.HandleStart("", func(cont containerd.Container, event *events.TaskStart) {
		s.containers[cont.ID()] = cont
	})
	if err != nil {
		return nil, err
	}
	err = s.watcher.HandleExit("", func(cont containerd.Container, event *events.TaskExit) {
		delete(s.containers, cont.ID())
	})
	if err != nil {
		return nil, err
	}
	return s, nil
}

// HandleCreate handles create events
func (s *State) HandleCreate(filter string, handler func(containerd.Container)) error {
	return s.watcher.HandleStart(filter, func(cont containerd.Container, event *events.TaskStart) {
		handler(cont)
	})
}

// HandleDelete handles delete events
func (s *State) HandleDelete(filter string, handler func(containerd.Container)) error {
	return s.watcher.HandleExit(filter, func(cont containerd.Container, event *events.TaskExit) {
		handler(cont)
	})
}

// Listen looks for already existing containers and watch for events
func (s *State) Listen(ctxw context.Context) error {
	// First loop over already known containers
	ctx := context.Background()
	ctxCont := context.Background()
	containers, err := s.watcher.Container.Containers(ctx, "")
	if err != nil {
		return err
	}
	for _, container := range containers {
		go func(id string) {
			contc, err := s.watcher.Container.LoadContainer(ctxCont, id)
			if err != nil {
				log.Error(err)
				return
			}
			info, err := contc.Info(ctxCont)
			if err != nil {
				log.Error(err)
				return
			}
			fmt.Println("info spec: ", info.Spec)
			for _, h := range s.watcher.startHandlers {
				h.handler(contc, nil)
			}
		}(container.ID())
	}
	// Then watch for docker events
	s.watcher.Listen(ctxw)
	return nil
}
