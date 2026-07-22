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
	"go-spring.org/spring/gs"
)

func init() {
	// The single Dubbo consumer (client) for the process, bound from the global
	// ${spring.dubbo.client} node. dubbo-go's config model has one consumer per
	// process (mirrored by dubbo-go.json's single "consumer" object), so unlike
	// most go-spring client starters this is a single default bean, not a map.
	// Per-stub tuning lives under ${spring.dubbo.client.references.<name>} and
	// overrides these defaults per reference (see reference.go). Gated on the
	// *Instance bean (no registries → no client) and the property; OnProperty is
	// a prefix check, so setting any spring.dubbo.client.* sub-key (including a
	// references entry) stands the client up.
	gs.Provide(
		NewClient,
		gs.IndexArg(0, gs.TagArg("${spring.dubbo.client}")),
	).Condition(gs.OnBean[*Instance](), gs.OnProperty("spring.dubbo.client"))
}

// ClientConfig is the consumer-level (client) configuration under the global
// ${spring.dubbo.client} node — the process-wide defaults every reference
// inherits unless it overrides per-stub. Every field is optional; empty/zero
// keeps dubbo-go's own default. (dubbo-go.json's consumer also exposes proxy /
// adaptive-service / tracing-key / max-wait-time-for-service-discovery, but v3
// has no clean client-level Option for them, so they are intentionally not
// surfaced here.)
type ClientConfig struct {
	Protocol    string        `value:"${protocol:=}"`      // dubbo(default)|tri|triple|jsonrpc
	Timeout     time.Duration `value:"${timeout:=}"`       // per-request timeout, e.g. "3s"
	RegistryIDs []string      `value:"${registry-ids:=}"`   // select global registries by ID; empty means all
	Filter      string        `value:"${filter:=}"`        // comma-separated filter chain; use "-name" to drop one from dubbo-go's default chain
	Params      map[string]string `value:"${params:=}"`    // escape hatch for consumer-level filter parameters
	Check       bool          `value:"${check:=true}"`     // false disables startup check (provider presence)
}

// NewClient builds the single *client.Client from the global ClientConfig and
// the shared *Instance. Registries are selected by RegistryIDs (empty means
// all); the client inherits the Instance's metrics and tracing. Per-stub
// overrides are applied later on each reference (see ReferenceConfig.options).
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
	if cfg.Filter != "" {
		opts = append(opts, client.WithClientFilter(cfg.Filter))
	}
	if len(cfg.Params) > 0 {
		opts = append(opts, client.WithClientParams(cfg.Params))
	}
	if !cfg.Check {
		opts = append(opts, client.WithClientNoCheck())
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
