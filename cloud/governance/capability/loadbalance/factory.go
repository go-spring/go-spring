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

// factory.go is the strategy seam: the [Provider] one implementation
// contributes, the [Factory] that collects them, and the built-in provider this
// package contributes.

package loadbalance

import (
	"fmt"
	"slices"

	"go-spring.org/stdlib/errutil"
)

// Resolver builds another strategy named by a [Selection]. [Factory.Create]
// supplies itself as the resolver, so a composing strategy (zone_aware) can build
// its delegate without a global registry.
type Resolver func(Selection) (Balancer, error)

// Provider is ONE strategy implementation — the built-in set, a company's
// strategy, ... It exports the strategy names it can create and builds a fresh,
// independent [Balancer] for one selection.
//
// selection is the whole selection the strategy was named by: a BUILT-IN provider
// reads its own typed parameter group out of it and ignores everything else,
// while a CONTRIBUTED provider reads the keys it owns out of [Selection.Params]
// and rejects the rest itself. Either way, a value the strategy does not read
// is simply not its business.
//
// resolve builds a strategy named by a nested selection; a simple strategy
// ignores it. A provider holds no per-target state; anything stateful belongs on
// the Balancer it returns.
//
// A name may be exported by exactly one provider; [Factory] enforces that at
// construction.
type Provider interface {
	Strategies() []string
	New(selection Selection, resolve Resolver) (Balancer, error)
}

// Factory holds the collected strategy providers and is the one place that
// builds balancers. It indexes the providers by strategy name once, at
// construction: Create is a map lookup, and a name exported by two providers is
// a wiring error caught then. A provider repeating one of its own names is
// harmless.
type Factory struct {
	providers map[string]Provider
}

// NewFactory collects providers into the name index. A strategy exported by
// more than one provider is a wiring error and panics.
func NewFactory(providers []Provider) *Factory {
	byName := make(map[string]Provider)
	for _, p := range providers {
		if p == nil {
			continue
		}
		own := make(map[string]struct{})
		for _, name := range p.Strategies() {
			if _, duplicate := own[name]; duplicate {
				continue
			}
			own[name] = struct{}{}
			if _, taken := byName[name]; taken {
				panic(fmt.Errorf("loadbalance: strategy %q exported by two providers", name))
			}
			byName[name] = p
		}
	}
	return &Factory{providers: byName}
}

// Create builds the strategy selection names. An empty name, an unknown name, or
// parameters the strategy rejects is an error — [Manager.Apply] turns it into a
// rejected document, so a bad rule never takes a client down: the last good
// configuration stays in force.
func (f *Factory) Create(selection Selection) (Balancer, error) {
	name := selection.Balancer
	if name == "" {
		return nil, errutil.Explain(nil, "loadbalance: balancer name is required")
	}
	p, ok := f.providers[name]
	if !ok {
		return nil, errutil.Explain(nil, "loadbalance: no strategy registered as %q (registered: %v)", name, f.names())
	}
	return p.New(selection, f.Create)
}

// names lists the exported strategy names, sorted. It is unexported: its one
// consumer is the "no such strategy" error below, and a strategy directory is
// not part of what this type promises callers.
func (f *Factory) names() []string {
	out := make([]string, 0, len(f.providers))
	for name := range f.providers {
		out = append(out, name)
	}
	slices.Sort(out)
	return out
}

// builtinStrategies are the names the built-in provider exports.
var builtinStrategies = []string{
	RoundRobin, LeastConn, ConsistentHash, Weighted, ZoneAware, Random, P2C,
}

// builtinProvider exports the strategies implemented in this package. It is
// unexported: the concrete type carries no meaning to callers, who only ever
// need the [Provider] value [NewBuiltinProvider] returns.
type builtinProvider struct{}

var _ Provider = builtinProvider{}

// NewBuiltinProvider returns the built-in strategy provider.
func NewBuiltinProvider() Provider { return builtinProvider{} }

// Strategies exports every built-in name.
func (builtinProvider) Strategies() []string { return builtinStrategies }

// New builds the built-in strategy named by selection. A built-in reads its own
// typed parameter group and ignores everything else — other groups and
// [Selection.Params] alike are not its business, so a stray value changes
// nothing rather than failing the build.
func (builtinProvider) New(selection Selection, resolve Resolver) (Balancer, error) {
	switch selection.Balancer {
	case RoundRobin:
		return newRoundRobin(selection)
	case LeastConn:
		return newLeastConn(selection)
	case ConsistentHash:
		return newConsistentHash(selection)
	case Weighted:
		return newWeighted(selection)
	case ZoneAware:
		return newZoneAware(selection, resolve)
	case Random:
		return newRandom(selection)
	case P2C:
		return newP2C(selection)
	}
	return nil, errutil.Explain(nil, "loadbalance: builtin provider does not implement %q", selection.Balancer)
}
