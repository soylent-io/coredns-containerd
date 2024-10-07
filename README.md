Container watch
===============

Listen containerd events throught containerd sockets, and do stuff.

Based on
- https://github.com/factorysh/containers-watch/
- https://github.com/kevinjqiu/coredns-dockerdiscovery/

Corefile
--------

    containerd [CONTAINERD_ENDPOINT] {
        domain DOMAIN_NAME
    }

How To Build
------------

```
$ git clone github.com/coredns/coredns
$ cd coredns
# append "containerd:github.com/soylent-io/coredns-containerd/containerd" after kubernetes entry in plugin.cfg
$ make
```

Building docker image
---------------------

In coredns/ directory with "coredns" built:

```
$ mkdir -p build/docker/amd64
$ cp coredns build/docker/amd64
$ make -f Makefile.docker DOCKER=soylentio LINUX_ARCH=amd64 VERSION=1.11.3-1 docker-build
```
