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
	"runtime"
	"time"

	"dubbo.apache.org/dubbo-go/v3/client"
	"go-spring.org/spring/gs"
)

func init() {
	gs.Provide(
		NewClient,
	).Condition(gs.OnBean[*Instance]())
}

// NewClient builds the single *client.Client from the shared *Instance's
// consumer configuration.
func NewClient(d *Instance) (*client.Client, error) {
	cfg := d.Consumer()
	var opts []client.ClientOption

	// Protocol.
	switch cfg.Protocol {
	case "tri", "triple":
		opts = append(opts, client.WithClientProtocolTriple())
	case "jsonrpc":
		opts = append(opts, client.WithClientProtocolJsonRPC())
	case "dubbo":
		opts = append(opts, client.WithClientProtocolDubbo())
	}

	// Consumer-level defaults (inherited by references).
	if cfg.RequestTimeout != "" {
		if d, err := time.ParseDuration(cfg.RequestTimeout); err == nil && d > 0 {
			opts = append(opts, client.WithClientRequestTimeout(d))
		}
	}
	if cfg.Filter != "" {
		opts = append(opts, client.WithClientFilter(cfg.Filter))
	}
	if !cfg.Check {
		opts = append(opts, client.WithClientNoCheck())
	}
	if len(cfg.RegistryIDs) > 0 {
		opts = append(opts, client.WithClientRegistryIDs(cfg.RegistryIDs...))
	}
	if cfg.Cluster != "" {
		opts = append(opts, client.WithClientClusterStrategy(cfg.Cluster))
	}
	if cfg.LoadBalance != "" {
		opts = append(opts, client.WithClientLoadBalance(cfg.LoadBalance))
	}
	if cfg.Retries > 0 {
		opts = append(opts, client.WithClientRetries(cfg.Retries))
	}
	if cfg.Group != "" {
		opts = append(opts, client.WithClientGroup(cfg.Group))
	}
	if cfg.Version != "" {
		opts = append(opts, client.WithClientVersion(cfg.Version))
	}
	if cfg.Serialization != "" {
		opts = append(opts, client.WithClientSerialization(cfg.Serialization))
	}
	if cfg.Sticky {
		opts = append(opts, client.WithClientSticky())
	}
	if cfg.ForceTag {
		opts = append(opts, client.WithClientForceTag())
	}

	// Registries: select from the global block.
	registries, err := selectRegistries(d.Registries(), cfg.RegistryIDs)
	if err != nil {
		return nil, err
	}
	for name, r := range registries {
		opts = append(opts, client.WithClientRegistry(r.options(name)...))
	}

	return d.NewClient(opts...)
}

// options translates DubboReference into dubbo-go client.ReferenceOption.
func (c DubboReference) options() []client.ReferenceOption {
	var opts []client.ReferenceOption
	if c.Protocol != "" {
		opts = append(opts, client.WithProtocol(c.Protocol))
	}
	if len(c.RegistryIDs) > 0 {
		opts = append(opts, client.WithRegistryIDs(c.RegistryIDs...))
	}
	if c.Filter != "" {
		opts = append(opts, client.WithFilter(c.Filter))
	}
	if c.Cluster != "" {
		opts = append(opts, client.WithCluster(c.Cluster))
	}
	if c.LoadBalance != "" {
		opts = append(opts, client.WithLoadBalance(c.LoadBalance))
	}
	if c.Timeout != "" {
		if d, err := time.ParseDuration(c.Timeout); err == nil && d > 0 {
			opts = append(opts, client.WithRequestTimeout(d))
		}
	}
	if c.Retries > 0 {
		opts = append(opts, client.WithRetries(c.Retries))
	}
	if c.Group != "" {
		opts = append(opts, client.WithGroup(c.Group))
	}
	if c.Version != "" {
		opts = append(opts, client.WithVersion(c.Version))
	}
	if c.Serialization != "" {
		opts = append(opts, client.WithSerialization(c.Serialization))
	}
	if len(c.Params) > 0 {
		opts = append(opts, client.WithParams(c.Params))
	}
	if c.Interface != "" {
		opts = append(opts, client.WithInterface(c.Interface))
	}
	if c.Check {
		opts = append(opts, client.WithCheck())
	}
	if c.URL != "" {
		opts = append(opts, client.WithURL(c.URL))
	}
	if c.Async {
		opts = append(opts, client.WithAsync())
	}
	if c.Generic {
		opts = append(opts, client.WithGeneric())
	}
	if c.Sticky {
		opts = append(opts, client.WithSticky())
	}
	if c.ForceTag {
		opts = append(opts, client.WithForceTag())
	}
	for name, m := range c.Methods {
		if m.Name == "" {
			m.Name = name
		}
		if mopts := m.options(); len(mopts) > 0 {
			opts = append(opts, client.WithMethod(mopts...))
		}
	}
	return opts
}

// RegisterReference registers a Triple-generated RPC stub as a bean.
// name is the key under ${spring.dubbo.consumer.references} to bind.
func RegisterReference[T any](name string, ctor func(*client.Client, ...client.ReferenceOption) (T, error)) {
	b := gs.Provide(func(cli *client.Client, cfg DubboReference) (T, error) {
		return ctor(cli, cfg.options()...)
	}, gs.IndexArg(1, gs.TagArg("${spring.dubbo.consumer.references."+name+"}")))
	if _, file, line, ok := runtime.Caller(1); ok {
		b.SetFileLine(file, line)
	}
}
