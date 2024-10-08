package containerd

import (
	"context"
	"errors"
	"net"
	"os"
	"sync"

	"github.com/containerd/containerd"
	"github.com/containerd/containerd/api/events"
	"github.com/containerd/containerd/cio"
	"github.com/containerd/containerd/namespaces"

	log "github.com/sirupsen/logrus"
	"github.com/soylent-io/coredns-containerd/watcher"

	//runtimeapi "k8s.io/cri-api/pkg/apis/runtime/v1"

	"github.com/vishvananda/netlink"
	"github.com/vishvananda/netns"

	"encoding/json"

	"github.com/opencontainers/runtime-spec/specs-go"

	"github.com/coredns/coredns/plugin"
	"github.com/coredns/coredns/request"
	"github.com/miekg/dns"
)

// ContainerdDiscovery is a plugin that conforms to the coredns plugin interface
type ContainerdDiscovery struct {
	Next               plugin.Handler
	containerdEndpoint string
	domain             string
	watcher            *watcher.Watcher
	//runtimeClient      runtimeapi.RuntimeServiceClient

	A    map[string]net.IP
	AAAA map[string]net.IP

	mutex sync.RWMutex
	ttl   uint32
}

// NewContainerdDiscovery constructs a new DockerDiscovery object
func NewContainerdDiscovery(containerdEndpoint, domain string) *ContainerdDiscovery {
	return &ContainerdDiscovery{
		containerdEndpoint: containerdEndpoint,
		domain:             domain,
		A:                  make(map[string]net.IP),
		AAAA:               make(map[string]net.IP),
		ttl:                3600,
	}
}

// Name implements plugin.Handler
func (cd *ContainerdDiscovery) Name() string {
	return "containerd"
}

// ServeDNS implements plugin.Handler
func (cd *ContainerdDiscovery) ServeDNS(ctx context.Context, w dns.ResponseWriter, r *dns.Msg) (int, error) {
	state := request.Request{W: w, Req: r}
	log.Infof("ServeDNS: QName %s", state.QName())

	cd.mutex.RLock()
	defer cd.mutex.RUnlock()

	var answers []dns.RR
	switch state.QType() {
	case dns.TypeA:
		val, ok := cd.A[state.QName()]
		if ok {
			answers = getAnswer(state.Name(), []net.IP{val}, cd.ttl, false)
		}
	case dns.TypeAAAA:
		val, ok := cd.AAAA[state.QName()]
		if ok {
			answers = getAnswer(state.Name(), []net.IP{val}, cd.ttl, true)
		} else {
			_, ok := cd.A[state.QName()]
			if ok {
				// in accordance with https://tools.ietf.org/html/rfc6147#section-5.1.2 we should return an empty answer section if no AAAA records are available and a A record is available when the client requested AAAA
				record := new(dns.AAAA)
				record.Hdr = dns.RR_Header{
					Name:     state.Name(),
					Rrtype:   dns.TypeAAAA,
					Class:    dns.ClassINET,
					Ttl:      cd.ttl,
					Rdlength: 0,
				}
				answers = append(answers, record)
			}
		}
	}

	if len(answers) == 0 {
		return plugin.NextOrFailure(cd.Name(), cd.Next, ctx, w, r)
	}

	m := new(dns.Msg)
	m.SetReply(r)
	m.Authoritative, m.RecursionAvailable, m.Compress = true, false, true
	m.Answer = answers

	state.SizeAndDo(m)
	m = state.Scrub(m)
	err := w.WriteMsg(m)
	if err != nil {
		log.Printf("[containerd] Error: %s", err.Error())
	}
	return dns.RcodeSuccess, nil
}

func (cd *ContainerdDiscovery) start() error {
	// List all containers in the namespace
	ctx := namespaces.WithNamespace(context.Background(), "k8s.io")
	containers, err := cd.watcher.Container.Containers(ctx)
	if err != nil {
		panic(err)
	}

	// Iterate through the containers and print their IDs and names
	for _, c := range containers {
		log.Infof("Found container: %s", c.ID())
		cd.updateContainerRR(c)
	}

	v, err := cd.watcher.Version()
	if err != nil {
		panic(err)
	}
	log.Infof("[containerd] start: %s", v)

	cd.watcher.HandleStart("", func(c containerd.Container, event *events.TaskStart) {
		log.Infof("Start: %s", c.ID())

		cd.updateContainerRR(c)
	})
	cd.watcher.HandleExit("", func(c containerd.Container, event *events.TaskExit) {
		log.Infof("Exit: %s", c.ID())
	})
	cd.watcher.Listen(context.Background())

	return errors.New("containerd event loop closed")
}

func (cd *ContainerdDiscovery) updateContainerRR(c containerd.Container) {
	cd.mutex.Lock()
	defer cd.mutex.Unlock()

	hostname, err := cd.getContainerHostname(c)
	if err != nil || hostname == "" {
		return
	}

	ipv4, err := cd.getContainerAddress(c)
	if err != nil {
		return
	}

	d := hostname + "." + cd.domain + "."
	cd.A[d] = ipv4
	log.Infof("%s A %s", d, ipv4)
}

// getAnswer function takes a slice of net.IPs and returns a slice of A/AAAA RRs.
func getAnswer(zone string, ips []net.IP, ttl uint32, v6 bool) []dns.RR {
	answers := []dns.RR{}
	for _, ip := range ips {
		if !v6 {
			record := new(dns.A)
			record.Hdr = dns.RR_Header{
				Name:   zone,
				Rrtype: dns.TypeA,
				Class:  dns.ClassINET,
				Ttl:    ttl,
			}
			record.A = ip
			answers = append(answers, record)
		} else if v6 {
			record := new(dns.AAAA)
			record.Hdr = dns.RR_Header{
				Name:   zone,
				Rrtype: dns.TypeAAAA,
				Class:  dns.ClassINET,
				Ttl:    ttl,
			}
			record.AAAA = ip
			answers = append(answers, record)
		}
	}
	return answers
}

func (cd *ContainerdDiscovery) getContainerAddress(c containerd.Container) (net.IP, error) {
	ip, err := cd.getContainerAddressCRI(c)
	if ip != nil {
		return ip, err
	}

	ip, err = cd.getContainerAddressCNI(c)
	if ip != nil {
		return ip, err
	}

	// Get the container task (process)
	task, err := c.Task(context.Background(), cio.Load)
	if err != nil {
		log.Error(err)
		return nil, err
	}

	// Get the container's network namespace (from its PID)
	pid := task.Pid()

	// Use netns to enter the container's network namespace
	netnsHandle, err := netns.GetFromPid(int(pid))
	if err != nil {
		log.Error(err)
		return nil, err
	}
	defer netnsHandle.Close()

	// Set the network namespace
	netns.Set(netnsHandle)

	// Use netlink to get the container's IP address
	links, err := netlink.LinkList()
	if err != nil {
		log.Error(err)
		return nil, err
	}

	for _, link := range links {
		addrs, err := netlink.AddrList(link, netlink.FAMILY_V4)
		if err != nil {
			log.Error(err)
			return nil, err
		}
		for _, addr := range addrs {
			if addr.Label == "lo" {
				continue
			}

			return addr.IP, nil
		}
	}

	return nil, nil
}

func (cd *ContainerdDiscovery) getContainerAddressCRI( /* c */ containerd.Container) (net.IP, error) {
	/*
		resp, err := cd.runtimeClient.PodSandboxStatus(context.Background(), &runtimeapi.PodSandboxStatusRequest{
			PodSandboxId: c.ID(),
		})
		if err != nil {
			log.Warnf("Failed to get pod sandbox status: %v", err)
			return nil, err
		}
		ip, _, err := net.ParseCIDR(resp.Status.Network.Ip)
		if err != nil {
			log.Warn(err)
			return nil, err
		}
		return ip, nil
	*/
	return nil, nil
}

func (cd *ContainerdDiscovery) getContainerAddressCNI(c containerd.Container) (net.IP, error) {
	cniResultFile := "/var/lib/cni/results/bridge-" + c.ID() + "-eth0"
	data, err := os.ReadFile(cniResultFile)
	if err != nil {
		if !os.IsNotExist(err) {
			log.Warn(err)
		}
		return nil, err
	}

	var res map[string]interface{}
	err = json.Unmarshal(data, &res)
	if err != nil {
		log.Warn(err)
		return nil, err
	}

	if result, ok := res["result"].(map[string]interface{}); ok {
		if ips, ok := result["ips"].([]interface{}); ok && len(ips) > 0 {
			if ip0, ok := ips[0].(map[string]interface{}); ok {
				if address, ok := ip0["address"].(string); ok {
					ip, _, err := net.ParseCIDR(address)
					if err != nil {
						log.Warn(err)
						return nil, err
					}
					return ip, nil
				}
			}
		}
	}

	return nil, nil
}

func (cd *ContainerdDiscovery) getContainerHostname(c containerd.Container) (string, error) {
	info, err := c.Info(context.Background())
	if err != nil {
		log.Error(err)
		return "", err
	}
	if info.Spec.GetTypeUrl() == "types.containerd.io/opencontainers/runtime-spec/1/Spec" {
		spec := specs.Spec{}
		err = json.Unmarshal(info.Spec.GetValue(), &spec)
		if err != nil {
			log.Error(err)
			return "", err
		}

		return spec.Hostname, nil
	}

	return "", nil
}
