/*
 * Copyright 2025 The Go-Spring Authors.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *      https://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package StarterDubbo

import (
	"context"
	"time"

	"dubbo.apache.org/dubbo-go/v3/protocol"
	"dubbo.apache.org/dubbo-go/v3/registry"
	"dubbo.apache.org/dubbo-go/v3/server"
	"go-spring.org/spring/gs"
	"go-spring.org/stdlib/errutil"
	"go-spring.org/stdlib/flatten"
)

func init() {
	enableSimpleDubboServer := gs.OnProperty("spring.dubbo.server.enabled").
		HavingValue("true").MatchIfMissing()
	gs.Module(enableSimpleDubboServer, func(r gs.BeanProvider, p flatten.Storage) error {

		// Register the Dubbo server
		// when a service register is available.
		r.Provide(
			NewDubboServer,
			gs.IndexArg(0, gs.TagArg("${spring.dubbo.server}")),
		).Export(gs.As[gs.Server]()).
			Condition(gs.OnBean[ServiceRegister]())
		return nil
	})
}

// ServiceRegister registers services on a Dubbo server.Server. Extracting the
// registration behind this function type keeps DubboServer service-agnostic:
// it drives the lifecycle while each service supplies its own register bean.
type ServiceRegister func(svr *server.Server) error

// ProtocolCfg configures a single Dubbo protocol listener. The map key under
// ${spring.dubbo.server.protocols} is the dubbo-go protocol name (e.g. "tri",
// "dubbo", "jsonrpc", "rest") and is passed straight through, so any protocol
// dubbo-go supports works without changing this starter. Empty/zero fields are
// treated as unset and their options are skipped.
type ProtocolCfg struct {
	Port   int               `value:"${port:=0}"`
	Ip     string            `value:"${ip:=}"`
	Params map[string]string `value:"${params:=}"` // extra protocol params, escape hatch
}

// RegistryCfg configures a single service registry. The map key under
// ${spring.dubbo.server.registries} is the dubbo-go registry name (e.g.
// "etcdv3", "nacos", "zookeeper", "polaris") and is passed straight through.
// Empty/zero fields are treated as unset and their options are skipped.
type RegistryCfg struct {
	Address    string            `value:"${address:=}"`
	Namespace  string            `value:"${namespace:=}"`
	Group      string            `value:"${group:=}"`
	Username   string            `value:"${username:=}"`
	Password   string            `value:"${password:=}"`
	Timeout    time.Duration     `value:"${timeout:=}"`  // e.g. "5s"
	TTL        time.Duration     `value:"${ttl:=}"`      // e.g. "15m"
	Weight     int64             `value:"${weight:=-1}"` // negative means unset; 0 is a valid weight
	Zone       string            `value:"${zone:=}"`
	Simplified bool              `value:"${simplified:=false}"`
	Preferred  bool              `value:"${preferred:=false}"` // try this registry first
	Params     map[string]string `value:"${params:=}"`
}

// Config defines Dubbo server configuration. Besides the map-driven protocols
// and registries, it exposes the common provider-level knobs; every field is
// optional and empty/zero values are skipped so dubbo-go keeps its own default.
//
// Enum-like fields accept the dubbo-go names:
//   - Cluster:       failover(default)|failfast|failsafe|failback|forking|available|broadcast|zoneAware
//   - LoadBalance:   random(default)|roundrobin|leastactive|consistenthashing|p2c
//   - Serialization: hessian2|protobuf|msgpack|json
type Config struct {
	// Provider-wide defaults applied to every exported service.
	Group           string        `value:"${group:=}"`
	Version         string        `value:"${version:=}"`
	Cluster         string        `value:"${cluster:=}"`
	LoadBalance     string        `value:"${load-balance:=}"`
	Serialization   string        `value:"${serialization:=}"`
	Retries         int           `value:"${retries:=-1}"` // negative means unset; 0 explicitly disables retries
	Filter          string        `value:"${filter:=}"`    // comma-separated filter names
	Token           string        `value:"${token:=}"`
	AccessLog       string        `value:"${access-log:=}"`
	Auth            string        `value:"${auth:=}"`
	Tag             string        `value:"${tag:=}"`
	Warmup          time.Duration `value:"${warmup:=}"` // e.g. "10m"
	NotRegister     bool          `value:"${not-register:=false}"`
	AdaptiveService bool          `value:"${adaptive-service:=false}"`

	// Protocols and registries are map-driven: only configured entries are
	// enabled, so one server can expose several protocols and publish to
	// several registries at once.
	Protocols  map[string]ProtocolCfg `value:"${protocols:=}"`
	Registries map[string]RegistryCfg `value:"${registries:=}"`
}

// DubboServer adapts a Dubbo-go server.Server to the Go-Spring server lifecycle.
type DubboServer struct {
	cfg  Config
	reg  ServiceRegister
	svr  *server.Server
	done chan struct{}
}

// NewDubboServer creates a DubboServer from ${spring.dubbo.server} configuration.
func NewDubboServer(cfg Config, reg ServiceRegister) *DubboServer {
	return &DubboServer{cfg: cfg, reg: reg, done: make(chan struct{})}
}

// buildOptions translates Config into dubbo-go server options. Every field is
// optional: empty/zero values are skipped so dubbo-go keeps its own defaults.
// When no protocol is configured, a Triple listener on port 20000 is used.
func (c *Config) buildOptions() []server.ServerOption {
	var opts []server.ServerOption

	// Provider-wide defaults.
	if c.Group != "" {
		opts = append(opts, server.WithServerGroup(c.Group))
	}
	if c.Version != "" {
		opts = append(opts, server.WithServerVersion(c.Version))
	}
	if c.Cluster != "" {
		opts = append(opts, server.WithServerCluster(c.Cluster))
	}
	if c.LoadBalance != "" {
		opts = append(opts, server.WithServerLoadBalance(c.LoadBalance))
	}
	if c.Serialization != "" {
		opts = append(opts, server.WithServerSerialization(c.Serialization))
	}
	if c.Retries >= 0 {
		opts = append(opts, server.WithServerRetries(c.Retries))
	}
	if c.Filter != "" {
		opts = append(opts, server.WithServerFilter(c.Filter))
	}
	if c.Token != "" {
		opts = append(opts, server.WithServerToken(c.Token))
	}
	if c.AccessLog != "" {
		opts = append(opts, server.WithServerAccesslog(c.AccessLog))
	}
	if c.Auth != "" {
		opts = append(opts, server.WithServerAuth(c.Auth))
	}
	if c.Tag != "" {
		opts = append(opts, server.WithServerTag(c.Tag))
	}
	if c.Warmup > 0 {
		opts = append(opts, server.WithServerWarmUp(c.Warmup))
	}
	if c.NotRegister {
		opts = append(opts, server.WithServerNotRegister())
	}
	if c.AdaptiveService {
		opts = append(opts, server.WithServerAdaptiveService())
	}

	// Protocol listeners.
	if len(c.Protocols) == 0 {
		// Default: a single Triple listener on port 20000.
		opts = append(opts, server.WithServerProtocol(
			protocol.WithID("tri"),
			protocol.WithProtocol("tri"),
			protocol.WithPort(20000),
		))
	} else {
		for name, pc := range c.Protocols {
			// The map key is the dubbo-go protocol name; WithID keeps each
			// entry distinct in the server's protocol map.
			pOpts := []protocol.ServerOption{
				protocol.WithID(name),
				protocol.WithProtocol(name),
			}
			if pc.Port > 0 {
				pOpts = append(pOpts, protocol.WithPort(pc.Port))
			}
			if pc.Ip != "" {
				pOpts = append(pOpts, protocol.WithIp(pc.Ip))
			}
			if len(pc.Params) > 0 {
				pOpts = append(pOpts, protocol.WithParams(pc.Params))
			}
			opts = append(opts, server.WithServerProtocol(pOpts...))
		}
	}

	// Registry publish targets.
	for name, rc := range c.Registries {
		// The map key is the dubbo-go registry name; WithID keeps each entry
		// distinct in the server's registry map.
		rOpts := []registry.Option{
			registry.WithID(name),
			registry.WithRegistry(name),
		}
		if rc.Address != "" {
			rOpts = append(rOpts, registry.WithAddress(rc.Address))
		}
		if rc.Namespace != "" {
			rOpts = append(rOpts, registry.WithNamespace(rc.Namespace))
		}
		if rc.Group != "" {
			rOpts = append(rOpts, registry.WithGroup(rc.Group))
		}
		if rc.Username != "" {
			rOpts = append(rOpts, registry.WithUsername(rc.Username))
		}
		if rc.Password != "" {
			rOpts = append(rOpts, registry.WithPassword(rc.Password))
		}
		if rc.Timeout > 0 {
			rOpts = append(rOpts, registry.WithTimeout(rc.Timeout))
		}
		if rc.TTL > 0 {
			rOpts = append(rOpts, registry.WithTTL(rc.TTL))
		}
		if rc.Weight >= 0 {
			rOpts = append(rOpts, registry.WithWeight(rc.Weight))
		}
		if rc.Zone != "" {
			rOpts = append(rOpts, registry.WithZone(rc.Zone))
		}
		if rc.Simplified {
			rOpts = append(rOpts, registry.WithSimplified())
		}
		if rc.Preferred {
			rOpts = append(rOpts, registry.WithPreferred())
		}
		if len(rc.Params) > 0 {
			rOpts = append(rOpts, registry.WithParams(rc.Params))
		}
		opts = append(opts, server.WithServerRegistry(rOpts...))
	}

	return opts
}

// Run assembles the Dubbo server from Config and starts serving once Go-Spring
// signals readiness. Dubbo's Serve blocks forever internally, so it runs in a
// goroutine while Run parks on the done channel; Stop closes done to hand
// control back to Go-Spring's shutdown sequence.
func (s *DubboServer) Run(ctx context.Context, sig gs.ReadySignal) error {
	svr, err := server.NewServer(s.cfg.buildOptions()...)
	if err != nil {
		return errutil.Explain(err, "failed to create dubbo server")
	}
	if err = s.reg(svr); err != nil {
		return errutil.Explain(err, "failed to register dubbo service")
	}
	s.svr = svr

	<-sig.TriggerAndWait()

	errCh := make(chan error, 1)
	go func() {
		// Serve exports the service (binding the listeners) and then blocks.
		errCh <- svr.Serve()
	}()

	select {
	case err = <-errCh:
		return errutil.Explain(err, "failed to serve dubbo server")
	case <-s.done:
		return nil
	}
}

// Stop signals Run to return so Go-Spring can complete its shutdown sequence.
func (s *DubboServer) Stop() error {
	close(s.done)
	return nil
}
