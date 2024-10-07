package containerd

import (
	"strconv"

	log "github.com/sirupsen/logrus"
	"github.com/soylent-io/coredns-containerd/watcher"

	//"google.golang.org/grpc"
	//"google.golang.org/grpc/credentials/insecure"
	//runtimeapi "k8s.io/cri-api/pkg/apis/runtime/v1"

	"github.com/coredns/caddy"
	"github.com/coredns/coredns/core/dnsserver"
	"github.com/coredns/coredns/plugin"
)

const defaultContainerdEndpoint = "/var/run/containerd/containerd.sock"
const defaultContainerdDomain = "node.local"

func init() {
	caddy.RegisterPlugin("containerd", caddy.Plugin{
		ServerType: "dns",
		Action:     setup,
	})
}

func createPlugin(c *caddy.Controller) (*ContainerdDiscovery, error) {
	cd := NewContainerdDiscovery(defaultContainerdEndpoint, defaultContainerdDomain)

	if c != nil {
		for c.Next() {
			args := c.RemainingArgs()
			if len(args) == 1 {
				cd.containerdEndpoint = args[0]
			}

			if len(args) > 1 {
				return cd, c.ArgErr()
			}

			for c.NextBlock() {
				var value = c.Val()
				switch value {
				case "domain":
					if !c.NextArg() {
						return cd, c.ArgErr()
					}
					cd.domain = c.Val()
				case "ttl":
					if !c.NextArg() {
						return cd, c.ArgErr()
					}
					ttl, err := strconv.ParseUint(c.Val(), 10, 32)
					if err != nil {
						return cd, err
					}
					if ttl > 0 {
						cd.ttl = uint32(ttl)
					}
				default:
					return cd, c.Errf("unknown property: '%s'", c.Val())
				}
			}
		}
	}

	watcher, err := watcher.New(cd.containerdEndpoint)
	if err != nil {
		return cd, err
	}
	cd.watcher = watcher

	/*
		conn, err := grpc.NewClient(cd.containerdEndpoint, grpc.WithTransportCredentials(insecure.NewCredentials()))
		if err != nil {
			return cd, err
		}
		cd.runtimeClient = runtimeapi.NewRuntimeServiceClient(conn)
	*/
	return cd, nil
}

func setup(c *caddy.Controller) error {
	cd, err := createPlugin(c)
	if err != nil {
		return err
	}
	go cd.start()

	dnsserver.GetConfig(c).AddPlugin(func(next plugin.Handler) plugin.Handler {
		cd.Next = next
		return cd
	})
	return nil
}

func Main() {
	log.SetLevel(log.DebugLevel)
	cd, err := createPlugin(nil)
	if err != nil {
		panic(err)
	}
	cd.start()
}
