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

// ReferenceConfig holds per-stub consumer tuning for a Triple reference, bound
// from ${spring.dubbo.client.references.<name>} (see RegisterReference below)
// and turned into dubbo-go client.ReferenceOption via options, passed to the
// stub constructor so the autowired stub honors them on every call.
//
// It is the reference-level counterpart to the global ${spring.dubbo.client}
// (see ClientConfig): every field here overrides the client-level default for
// this one stub, which is the only place per-stub differences are expressed now
// that the client is a single process-wide bean (mirroring dubbo-go.json's
// consumer/consumer.references shape). All fields are optional; empty/zero
// keeps dubbo-go's own default (or the client-level default for inherited
// fields like protocol/registry-ids/filter).
//
// Enum-like fields accept the dubbo-go names:
//   - Cluster:     failover(default)|failfast|failsafe|failback|forking|available|broadcast|zoneAware
//   - LoadBalance: random(default)|roundrobin|leastactive|consistenthashing|p2c
//
// Note: retries only takes effect with cluster=failover (the default cluster).
type ReferenceConfig struct {
	Protocol    string            `value:"${protocol:=}"`     // dubbo(default)|tri|triple|jsonrpc; overrides client-level
	RegistryIDs []string          `value:"${registry-ids:=}"` // select global registries by ID; overrides client-level
	Filter      string            `value:"${filter:=}"`       // comma-separated filter chain; overrides client-level
	Timeout     time.Duration     `value:"${timeout:=}"`      // per-request timeout, e.g. "3s"; overrides client-level
	Retries     int               `value:"${retries:=-1}"`    // -1 keeps dubbo-go default; 0 disables; >0 retries that many times
	Cluster     string            `value:"${cluster:=}"`      // cluster strategy
	LoadBalance string            `value:"${load-balance:=}"` // load-balance strategy
	Params      map[string]string `value:"${params:=}"`       // escape hatch for reference-level filter parameters
}

// options translates ReferenceConfig into the dubbo-go ReferenceOption list
// passed to a stub constructor via RegisterReference. Empty/zero fields are
// skipped so dubbo-go keeps its own (or the client-level) default.
func (c ReferenceConfig) options() []client.ReferenceOption {
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
	if c.Timeout > 0 {
		opts = append(opts, client.WithRequestTimeout(c.Timeout))
	}
	if c.Retries >= 0 {
		opts = append(opts, client.WithRetries(c.Retries))
	}
	if c.Cluster != "" {
		opts = append(opts, client.WithCluster(c.Cluster))
	}
	if c.LoadBalance != "" {
		opts = append(opts, client.WithLoadBalance(c.LoadBalance))
	}
	if len(c.Params) > 0 {
		opts = append(opts, client.WithParams(c.Params))
	}
	return opts
}

// RegisterReference registers a Triple-generated RPC stub (the consumer-side
// "reference") as a bean, so business beans autowire the typed stub instead of
// rebuilding it on every call. It works for any stub: pass its code-generated
// constructor (e.g. greet.NewGreetService), which every Triple stub exposes with
// the same shape - func(*client.Client, ...client.ReferenceOption) (T, error).
//
// The starter only provides this helper; the app calls it explicitly for each
// stub. Unlike the *client.Client bean (which the starter auto-builds from the
// global ${spring.dubbo.client}, because that type is generic), a typed stub
// and its constructor are app-specific generated code the starter cannot see -
// so references cannot be auto-registered. Registration staying in the app is
// what keeps the autowire typed.
//
// Two args are injected into the stub constructor, parameterized only by name:
//   - arg 0: the single global *client.Client bean, autowired by type (no tag,
//     matching how NewSimpleDubboServer injects *Instance). Dubbo-go allows
//     one consumer per process, so there is exactly one client bean.
//   - arg 1: a ReferenceConfig bound from ${spring.dubbo.client.references.<name>}
//     (gs.TagArg("${spring.dubbo.client.references.<name>}")), turned into
//     client.ReferenceOption via ReferenceConfig.options.
//
// Because every stub shares the same ReferenceConfig shape, the per-reference
// knobs (protocol/registry-ids/filter/timeout/retries/cluster/load-balance) are
// configured identically for every stub - only <name> and the constructor
// differ. The registered bean has the stub's interface type T, so a business
// bean autowires it by type (the layering real apps use: business bean -> RPC
// stub -> client).
//
// Must be called before gs.Run() (e.g. in main or an init), like gs.Provide.
func RegisterReference[T any](name string, ctor func(*client.Client, ...client.ReferenceOption) (T, error)) {
	b := gs.Provide(func(cli *client.Client, cfg ReferenceConfig) (T, error) {
		return ctor(cli, cfg.options()...)
	}, gs.IndexArg(1, gs.TagArg("${spring.dubbo.client.references."+name+"}")))
	// gs.Provide records its own call site (inside this helper) as the bean's
	// source location. Override it with the app's call site so the bean points
	// where RegisterReference was invoked - transparent vs. calling gs.Provide
	// inline, which matters now that this is a shared library helper.
	if _, file, line, ok := runtime.Caller(1); ok {
		b.SetFileLine(file, line)
	}
}
