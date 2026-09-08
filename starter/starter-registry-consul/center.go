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

package StarterRegistryConsul

import (
	"context"
	"time"

	"github.com/hashicorp/consul/api"
	"go-spring.org/cloud/discovery"
	"go-spring.org/log"
	"go-spring.org/spring/conf"
	"go-spring.org/spring/gs"
	"go-spring.org/stdlib/errutil"
	"go-spring.org/stdlib/flatten"
)

// consulCenter owns the ONE shared *api.Client for the agent configured under
// ${spring.registry.consul}. Both halves of the naming idiom derive from it:
// the registrar (the write side, starter.go) and the discovery backend
// auto-derived for the same agent (below) — the same "one center config, one
// connection" convergence as starter-registry-etcd's center.
//
// The Consul api client holds no resources that need releasing (it is an HTTP
// client wrapper with no Close), so the center carries no bean destructor;
// neither derived side closes the client either.
type consulCenter struct {
	client *api.Client
	config ConsulConfig
}

// newConsulCenter builds the shared client and probes the agent. The probe is
// the fail-fast both sides relied on when they built their own clients: a
// misconfigured or unreachable agent fails startup here, once.
func newConsulCenter(c ConsulConfig) (*consulCenter, error) {
	if c.Address == "" {
		return nil, errutil.Explain(nil, "registry-consul: address is required")
	}
	client, err := newConsulClient(c.Address, c.Scheme, c.Datacenter, c.Token, c.Namespace)
	if err != nil {
		return nil, err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if _, _, err := client.Catalog().Services((&api.QueryOptions{}).WithContext(ctx)); err != nil {
		return nil, errutil.Explain(err, "registry-consul: startup probe failed for %s", c.Address)
	}
	return &consulCenter{client: client, config: c}, nil
}

// newConsulClient builds a Consul api client for the given connection fields.
// It is the single construction path behind the center, the registrar fallback,
// and standalone discovery blocks.
func newConsulClient(address, scheme, datacenter, token, namespace string) (*api.Client, error) {
	client, err := api.NewClient(&api.Config{
		Address:    address,
		Scheme:     scheme,
		Datacenter: datacenter,
		Token:      token,
		Namespace:  namespace,
	})
	if err != nil {
		log.Errorf(context.Background(), log.TagAppDef, "create consul client for address=%s failed: %v", address, err)
		return nil, errutil.Explain(err, "registry-consul: create client for %s", address)
	}
	return client, nil
}

func init() {
	// The center bean exists exactly when the agent is configured. It is
	// nullable-injected (TagArg("?")) by the registrar Server and by discovery
	// backends that inherit the center connection.
	//
	// When discovery-name is non-empty (the default "consul"), the module also
	// derives a discovery backend bean for the same agent under that label —
	// the "one config block serves both halves" default. A dual-role app then
	// cites discovery=consul without any ${spring.discovery.consul} block; a
	// label colliding with another bean name fails loudly in the container.
	gs.Module(gs.OnProperty("spring.registry.consul.address"), func(r gs.BeanProvider, p flatten.Storage) error {
		var c ConsulConfig
		if err := conf.Bind(p, &c, "${spring.registry.consul}"); err != nil {
			return errutil.Explain(err, "registry-consul: bind center config")
		}
		r.Provide(newConsulCenter,
			gs.IndexArg(0, gs.ValueArg(c)),
		).Caller(1)

		if name := c.DiscoveryName; name != "" {
			r.Provide(newCenterDiscoveryBackend,
				gs.IndexArg(0, gs.TagArg("?")),
			).Name(name).Caller(1)
		}
		return nil
	})
}

// newCenterDiscoveryBackend serves snapshots for the center's agent through
// the shared client. It is the discovery bean auto-derived from
// ${spring.registry.consul}; it resolves the same agent the registrar writes
// into, so read and write can never diverge.
func newCenterDiscoveryBackend(cc *consulCenter) (discovery.Discovery, error) {
	if cc == nil {
		return nil, errutil.Explain(nil, "registry-consul: center client unavailable")
	}
	log.Debugf(context.Background(), log.TagAppDef, "derived consul discovery backend from center config, address=%s", cc.config.Address)
	return newConsulDiscovery(cc.client, ""), nil
}
