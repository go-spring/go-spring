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

package loadbalance

import (
	"maps"
	"slices"

	"go-spring.org/stdlib/errutil"
)

// Directory is the name-keyed table of [Factory] strategies — the runtime form
// of "the strategies this process knows". The built-in strategies are always
// present; a deployment extends it by contributing named [Factory] beans (see
// [NewManager]), which is what makes load balancing specializable without
// touching this package.
//
// A Directory is built once, when the manager is constructed, and read-only
// afterwards, so it is safe for concurrent use.
type Directory interface {
	// Build constructs the strategy named name from p. An unknown name, or a
	// params bag the strategy rejects, is an error.
	Build(name string, p *Params) (Balancer, error)

	// Names lists the registered strategy names, sorted.
	Names() []string
}

// directory is the concrete [Directory]: the built-in strategies merged with
// whatever factories a deployment contributed.
type directory struct {
	factories map[string]Factory
}

// NewDirectory returns a [Directory] over the built-in strategies extended by
// extra, keyed by name. An empty name, a nil factory, or a name that collides
// with a built-in strategy is an error — a deployment must not silently shadow
// round_robin. A nil or empty extra yields the built-ins alone.
func NewDirectory(extra map[string]Factory) (Directory, error) {
	factories := builtinFactories()
	for _, name := range slices.Sorted(maps.Keys(extra)) {
		f := extra[name]
		if name == "" {
			return nil, errutil.Explain(nil, "loadbalance: factory with empty name")
		}
		if f == nil {
			return nil, errutil.Explain(nil, "loadbalance: nil factory for %s", name)
		}
		if _, ok := factories[name]; ok {
			return nil, errutil.Explain(nil, "loadbalance: factory %q shadows a built-in strategy", name)
		}
		factories[name] = f
	}
	return &directory{factories: factories}, nil
}

// builtinDirectory returns the [Directory] of built-in strategies alone. It
// cannot fail, so callers that add nothing need no error path.
func builtinDirectory() Directory {
	return &directory{factories: builtinFactories()}
}

// BuiltinDirectory returns a [Directory] over the built-in strategies alone. It
// is what an adapter that builds its own balancers — a gRPC balancer registry
// entry, say — resolves a strategy name against when it has no contributed
// factories to add. It is deliberately a fresh, stateless value: a process that
// wants contributed strategies reaches for the governance manager's directory
// instead.
func BuiltinDirectory() Directory { return builtinDirectory() }

// Build constructs the strategy named name, delegating to its factory. A nil p
// is treated as the empty bag, so a caller that has no parameters (a composing
// factory building a sibling, say) need not wrap one.
func (d *directory) Build(name string, p *Params) (Balancer, error) {
	f, ok := d.factories[name]
	if !ok {
		return nil, errutil.Explain(nil, "loadbalance: no strategy registered as %q (registered: %v)", name, d.Names())
	}
	if p == nil {
		p = NewParams(nil)
	}
	return f.Build(d, p)
}

// Names lists the registered strategy names, sorted.
func (d *directory) Names() []string {
	return slices.Sorted(maps.Keys(d.factories))
}
