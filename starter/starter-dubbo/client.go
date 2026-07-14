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
	"time"

	"dubbo.apache.org/dubbo-go/v3/client"
	"go-spring.org/spring/conf"
	"go-spring.org/spring/gs"
	"go-spring.org/stdlib/flatten"
)

func init() {
	// Default client (bean "__default__"), gated on the *Instance bean: without a
	// registry there is nothing to discover, so no Instance means no client. The
	// optional ${spring.dubbo.client} block carries only protocol/timeout/
	// registry-ids; registries and observability come from the injected *Instance.
	gs.Provide(
		NewClient,
		gs.IndexArg(0, gs.TagArg("${spring.dubbo.client}")),
	).Condition(gs.OnBean[*Instance]())

	// Named instances (bean name = map key). Hand-rolled instead of gs.Group so
	// each binds its own ClientConfig while sharing the injected *Instance. Gated
	// only on the instances property; an instance declared without registries
	// fails fast on the missing *Instance dependency rather than being skipped.
	gs.Module(gs.OnProperty("spring.dubbo.client.instances"),
		func(r gs.BeanProvider, p flatten.Storage) error {
			var params struct {
				Instances map[string]ClientConfig `value:"${spring.dubbo.client.instances}"`
			}
			if err := conf.Bind(p, &params); err != nil {
				return err
			}
			for name, cfg := range params.Instances {
				r.Provide(
					NewClient,
					gs.IndexArg(0, gs.ValueArg(cfg)),
				).Name(name) // configured explicitly; the module gate is the only condition
			}
			return nil
		})
}

// ClientConfig defines the client-role configuration under ${spring.dubbo.client}
// (for the default client) or ${spring.dubbo.client.instances.<name>} (for a
// named instance). Every field is optional.
type ClientConfig struct {
	Protocol    string        `value:"${protocol:=}"`     // dubbo(default)|tri|triple|jsonrpc
	Timeout     time.Duration `value:"${timeout:=}"`      // per-request timeout, e.g. "3s"
	RegistryIDs []string      `value:"${registry-ids:=}"` // select global registries by ID; empty means all
}

// NewClient builds a *client.Client from a ClientConfig and the shared *Instance,
// serving both the default client and every named instance. Registries are
// selected by RegistryIDs (empty means all); the client inherits the Instance's
// metrics and tracing.
func NewClient(cfg ClientConfig, d *Instance) (*client.Client, error) {
	var opts []client.ClientOption

	switch cfg.Protocol {
	case "tri", "triple":
		opts = append(opts, client.WithClientProtocolTriple())
	case "jsonrpc":
		opts = append(opts, client.WithClientProtocolJsonRPC())
	default: // "" or "dubbo"
		opts = append(opts, client.WithClientProtocolDubbo())
	}

	if cfg.Timeout > 0 {
		opts = append(opts, client.WithClientRequestTimeout(cfg.Timeout))
	}
	registries, err := selectRegistries(d.Registries(), cfg.RegistryIDs)
	if err != nil {
		return nil, err
	}
	for name, r := range registries {
		opts = append(opts, client.WithClientRegistry(r.options(name)...))
	}

	return d.NewClient(opts...)
}
