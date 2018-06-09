package watcher

import (
	"github.com/docker/docker/api/types"
)

// ContainerAdaptor is an Adaptator for ContainerJSON
type ContainerAdaptor struct {
	container *types.ContainerJSON
}

// NewContainerAdaptor return an Adaptator
func NewContainerAdaptor(container *types.ContainerJSON) *ContainerAdaptor {
	return &ContainerAdaptor{
		container: container,
	}
}

// Field implements Adaptator interface
func (ca *ContainerAdaptor) Field(fieldpath []string) (value string, present bool) {
	if len(fieldpath) == 0 {
		return "", false
	}
	if fieldpath[0] == "docker" {
		if len(fieldpath) == 1 {
			return "", false
		}
		if fieldpath[1] == "label" {
			if len(fieldpath) == 2 {
				return "", false
			}
			v, ok := ca.container.Config.Labels[fieldpath[2]]
			return v, ok
		}
	}
	return "", false
}
