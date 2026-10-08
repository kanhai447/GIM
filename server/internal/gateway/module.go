package gateway

import (
	"errors"
	"net"
	"net/http"
	"time"

	"github.com/kanhai447/GIM/server/internal/gateway/authclient"
	gatewayconfig "github.com/kanhai447/GIM/server/internal/gateway/config"
	"github.com/kanhai447/GIM/server/internal/gateway/proxy"
	platformconfig "github.com/kanhai447/GIM/server/internal/platform/config"
	"github.com/kanhai447/GIM/server/internal/platform/discovery"
	platformetcd "github.com/kanhai447/GIM/server/internal/platform/etcd"
)

type Module struct {
	Config   gatewayconfig.Config
	Handler  http.Handler
	Resolver *discovery.Resolver
	Registry *discovery.Registry
}

func New(values platformconfig.Values, etcdClient *platformetcd.Client) (*Module, error) {
	if etcdClient == nil || etcdClient.Raw() == nil {
		return nil, errors.New("etcd client is not initialized")
	}
	cfg, err := gatewayconfig.FromValues(values)
	if err != nil {
		return nil, err
	}
	backend := discovery.NewEtcdBackend(etcdClient.Raw())
	resolver := discovery.NewResolver(backend)
	registry, err := discovery.NewDefaultRegistry(etcdClient.Raw())
	if err != nil {
		return nil, err
	}
	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.DialContext = (&net.Dialer{Timeout: cfg.DiscoveryTimeout, KeepAlive: 30 * time.Second}).DialContext
	transport.ResponseHeaderTimeout = cfg.ProxyTimeout
	auth := authclient.New(resolver, &http.Client{Transport: transport}, cfg.AuthTimeout)
	forwarder := proxy.New(transport, cfg.ProxyTimeout)
	return &Module{
		Config: cfg, Handler: NewHandler(auth, resolver, forwarder, cfg.DiscoveryTimeout), Resolver: resolver, Registry: registry,
	}, nil
}
