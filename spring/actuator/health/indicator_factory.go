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

package health

import (
	"context"
)

// NewIndicator builds an Indicator from a name and a probe function, replacing
// the near-identical little "{ name string; client X }" structs that each
// client starter would otherwise declare just to satisfy the interface.
//
// The probe should perform a bounded check (honoring ctx) and return nil when
// the component is usable. Use the options to declare probe groups or mark the
// indicator non-critical; without them the package defaults apply (readiness +
// startup, critical).
//
// Example:
//
//	ind := health.NewIndicator("redis:"+name, func(ctx context.Context) error {
//	    return client.Ping(ctx).Err()
//	})
func NewIndicator(name string, probe func(ctx context.Context) error, opts ...IndicatorOption) Indicator {
	ind := indicator{name: name, probe: probe, critical: true}
	for _, opt := range opts {
		opt(&ind)
	}
	// Only expose the optional Grouped interface when groups were explicitly
	// set. Otherwise return the bare indicator so GroupsOf applies its
	// default (readiness + startup) instead of seeing an empty group list.
	if len(ind.groups) > 0 {
		return &groupedIndicator{indicator: ind}
	}
	return &ind
}

// IndicatorOption customizes an indicator built by NewIndicator.
type IndicatorOption func(*indicator)

// WithGroups declares which probe groups the indicator contributes to,
// implementing Grouped. Without it the package defaults apply (readiness +
// startup).
func WithGroups(groups ...Group) IndicatorOption {
	return func(i *indicator) { i.groups = groups }
}

// NonCritical marks the indicator as non-critical, implementing Critical so a
// DOWN result is reported but does not fail the aggregate probe. Use it for a
// degraded-but-tolerable dependency that should not take the pod out of
// rotation.
func NonCritical() IndicatorOption {
	return func(i *indicator) { i.critical = false }
}

// indicator is the shared Indicator implementation. It always implements
// Critical (whose default of true matches the package default); Grouped is
// added by groupedIndicator only when groups are set.
type indicator struct {
	name     string
	probe    func(ctx context.Context) error
	groups   []Group
	critical bool
}

func (i *indicator) HealthName() string { return i.name }

func (i *indicator) CheckHealth(ctx context.Context) error { return i.probe(ctx) }

func (i *indicator) IsCritical() bool { return i.critical }

// groupedIndicator adds Grouped so an indicator with explicit groups is
// consulted for exactly those probes.
type groupedIndicator struct {
	indicator
}

func (i *groupedIndicator) HealthGroups() []Group { return i.groups }
