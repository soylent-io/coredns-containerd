package watcher

import (
	"testing"

	"github.com/containerd/containerd/filters"
	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/stretchr/testify/assert"
)

func TestFilter(t *testing.T) {
	container := &types.ContainerJSON{
		Config: &container.Config{
			Labels: map[string]string{
				"beuha": "aussi",
			},
		},
	}

	ca := NewContainerAdaptor(container)
	p, err := filters.Parse("docker.label.beuha==aussi")
	assert.NoError(t, err)
	ok := p.Match(ca)
	assert.True(t, ok)
	p, err = filters.Parse("docker.label.beuha==oups")
	assert.NoError(t, err)
	ok = p.Match(ca)
	assert.False(t, ok)
}
