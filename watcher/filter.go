package watcher

import (
	"strings"

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
		switch fieldpath[1] {
		case "config":
			if len(fieldpath) == 2 {
				return "", false
			}
			switch fieldpath[2] {
			case "hostname":
				return ca.container.Config.Hostname, true
			case "domainname":
				return ca.container.Config.Domainname, true
			case "user":
				return ca.container.Config.User, true
			case "image":
				return ca.container.Config.Image, true
			case "env":
				if len(fieldpath) == 3 {
					return "", false
				}
				for _, e := range ca.container.Config.Env {
					kv := strings.Split(e, "=")
					if kv[0] == fieldpath[3] {
						return strings.Join(kv[1:], "="), true
					}
				}
				return "", false
			case "labels":
				if len(fieldpath) == 3 {
					return "", false
				}
				v, ok := ca.container.Config.Labels[fieldpath[3]]
				return v, ok
			default:
				return "", false
			}
		}
	}
	return "", false
}
