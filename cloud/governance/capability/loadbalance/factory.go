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

// factory.go is the strategy seam: the [Provider] interface a strategy
// implementation contributes, the [Factory] that collects them, and the built-in
// provider this package contributes.

package loadbalance

import (
	"fmt"
	"slices"

	"go-spring.org/stdlib/errutil"
)

// Provider is ONE strategy implementation — the built-in set, a company's
// strategy, ... A [Factory] collects providers and routes each strategy name to
// the provider that exports it.
//
// selection is the whole selection the strategy was named by: a BUILT-IN provider
// reads its own strategy-prefixed keys out of it and ignores everything else,
// while a CONTRIBUTED provider reads the keys it owns out of [Selection.Params]
// and rejects the rest itself. Either way, a value the strategy does not read is
// simply not its business.
//
// A provider holds no per-target state; anything stateful belongs on the
// Balancer it returns.
type Provider interface {
	// Strategies returns the strategy names this provider exports. A name may be
	// exported by exactly one provider; [Factory] enforces that at construction.
	Strategies() []string

	// New builds a fresh, independent [Balancer] for the selection the strategy
	// was named by.
	New(selection Selection) (Balancer, error)
}

// Factory holds the collected strategy providers and is the one place that
// builds balancers. It indexes the providers by strategy name once, at
// construction, so Create is a map lookup. A provider repeating one of its own
// names is harmless.
type Factory struct {
	// providers maps each exported strategy name to the provider that builds it.
	providers map[string]Provider
}

// NewFactory collects providers into the name index. A strategy exported by
// more than one provider is a wiring error and panics; nil providers are
// ignored.
func NewFactory(providers []Provider) *Factory {
	byName := make(map[string]Provider)
	for _, p := range providers {
		if p == nil {
			continue
		}
		ownNames := make(map[string]struct{})
		for _, name := range p.Strategies() {
			if _, duplicate := ownNames[name]; duplicate {
				continue
			}
			ownNames[name] = struct{}{}
			if _, taken := byName[name]; taken {
				panic(fmt.Errorf("loadbalance: strategy %q exported by two providers", name))
			}
			byName[name] = p
		}
	}
	return &Factory{providers: byName}
}

// Create builds the strategy the selection names, then wraps it with the
// zone-aware filter when the selection asks for locality. An empty name, an
// unknown name, or parameters the strategy rejects is an error — [Manager.Apply]
// turns it into a rejected document, so a bad rule never takes a client down:
// the last good configuration stays in force.
func (f *Factory) Create(selection Selection) (Balancer, error) {
	name := selection.Balancer
	if name == "" {
		return nil, errutil.Explain(nil, "loadbalance: balancer name is required")
	}
	p, ok := f.providers[name]
	if !ok {
		return nil, errutil.Explain(nil, "loadbalance: no strategy registered as %q (registered: %v)", name, f.strategyNames())
	}
	bal, err := p.New(selection)
	if err != nil {
		return nil, err
	}
	// Locality is a filter over whatever strategy was named, so it is applied
	// here — one owner, every provider included — rather than being one strategy
	// among them.
	if selection.ZoneAware {
		bal, err = NewZoneAware(bal, selection.ZoneKey)
		if err != nil {
			return nil, err
		}
	}
	return bal, nil
}

// strategyNames lists the exported strategy names, sorted. It is unexported: its
// one consumer is the unknown-strategy error in [Factory.Create], and a strategy
// directory is not part of what this type promises callers.
func (f *Factory) strategyNames() []string {
	out := make([]string, 0, len(f.providers))
	for name := range f.providers {
		out = append(out, name)
	}
	slices.Sort(out)
	return out
}

// builtinStrategyNames lists the names the built-in provider exports.
var builtinStrategyNames = []string{
	RoundRobin, LeastConn, ConsistentHash, Weighted, Random, P2C,
}

// builtinProvider exports the strategies implemented in this package. It is
// unexported: the concrete type carries no meaning to callers, who only ever
// need the [Provider] value [NewBuiltinProvider] returns.
type builtinProvider struct{}

var _ Provider = builtinProvider{}

// NewBuiltinProvider returns the built-in strategy provider.
func NewBuiltinProvider() Provider { return builtinProvider{} }

// Strategies exports every built-in name.
func (builtinProvider) Strategies() []string { return builtinStrategyNames }

// New builds the built-in strategy named by selection. A built-in reads only its
// own strategy-prefixed keys and ignores everything else — other strategies'
// keys and [Selection.Params] alike are not its business, so a stray value
// changes nothing rather than failing the build.
func (builtinProvider) New(selection Selection) (Balancer, error) {
	switch selection.Balancer {
	case RoundRobin:
		return NewRoundRobin(selection), nil
	case LeastConn:
		return NewLeastConn(selection), nil
	case ConsistentHash:
		return NewConsistentHash(selection), nil
	case Weighted:
		return NewWeighted(selection), nil
	case Random:
		return NewRandom(selection), nil
	case P2C:
		return NewP2C(selection), nil
	}
	return nil, errutil.Explain(nil, "loadbalance: builtin provider does not implement %q", selection.Balancer)
}
