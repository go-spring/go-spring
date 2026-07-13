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
	// Default client (bean "__default__"), created when any registry is
	// resolvable. Role-first with fallback to the global registries.
	gs.Provide(
		NewClient,
		gs.IndexArg(0, gs.TagArg("${spring.dubbo.client}")),
		gs.IndexArg(1, gs.TagArg("${spring.dubbo.registries:=}")),
	).Condition(gs.Or(
		gs.OnProperty("spring.dubbo.registries"),
		gs.OnProperty("spring.dubbo.client.registries"),
	))

	// Named instances (bean name = map key). Hand-rolled instead of gs.Group so
	// the global registries can be injected as a second arg, giving instances
	// the same fallback as the default client.
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
					gs.IndexArg(1, gs.TagArg("${spring.dubbo.registries:=}")),
				).Name(name)
			}
			return nil
		})
}

// ClientConfig defines the client-role configuration under ${spring.dubbo.client}
// (for the default client) or ${spring.dubbo.client.instances.<name>} (for a
// named instance). Every field is optional.
type ClientConfig struct {
	Protocol    string                 `value:"${protocol:=}"`     // dubbo(default)|tri|triple|jsonrpc
	Timeout     time.Duration          `value:"${timeout:=}"`      // per-request timeout, e.g. "3s"
	RegistryIDs []string               `value:"${registry-ids:=}"` // select which registries (by ID) to use; empty means all
	Registries  map[string]RegistryCfg `value:"${registries:=}"`
}

// NewClient builds a *client.Client from a ClientConfig and the global
// registries, applying role-first/global-fallback. Serves both the default
// client and every named instance.
func NewClient(cfg ClientConfig, global map[string]RegistryCfg) (*client.Client, error) {
	return buildClient(cfg, resolveRegistries(global, cfg.Registries))
}

// buildClient translates a ClientConfig plus resolved registries into a
// dubbo-go *client.Client. Registry resolution happens lazily at Dial time.
func buildClient(cfg ClientConfig, registries map[string]RegistryCfg) (*client.Client, error) {
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
	if len(cfg.RegistryIDs) > 0 {
		opts = append(opts, client.WithClientRegistryIDs(cfg.RegistryIDs...))
	}
	for name, rc := range registries {
		opts = append(opts, client.WithClientRegistry(rc.options(name)...))
	}

	return client.NewClient(opts...)
}
