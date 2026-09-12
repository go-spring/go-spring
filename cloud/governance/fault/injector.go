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

package fault

import (
	"math/rand/v2"
	"sync/atomic"
	"time"
)

// Injector holds the live [Config] behind an atomic pointer so the center can
// hot-swap it from a source push without taking a lock on the
// execute path. A zero-valued config (or Enabled false) injects nothing, so an
// Injector with the zero Config is a transparent no-op.
type Injector struct {
	cfg      atomic.Pointer[Config]
	firstAt  atomic.Int64 // unix-nanos of the first affected call; 0 = none yet
	affected atomic.Int64 // count of calls that received any fault effect
}

// NewInjector returns an Injector holding c. The config can be replaced later
// with [Injector.SetConfig].
func NewInjector(c Config) *Injector {
	in := &Injector{}
	in.cfg.Store(&c)
	return in
}

// SetConfig atomically swaps the live config. It is safe to call concurrently
// with Execute; each call observes the config pointer current at its own
// [Injector.maybe] read, so a toggle takes effect on the next operation.
func (in *Injector) SetConfig(c Config) {
	in.cfg.Store(&c)
}

// Config returns a snapshot of the live config (the zero Config if none has
// been set, which is a no-op).
func (in *Injector) Config() Config {
	if c := in.cfg.Load(); c != nil {
		return *c
	}
	return Config{}
}

// maybe decides the fault applied to one call to resource:
//   - sleep is a latency to apply first (always, when Latency > 0);
//   - inject reports whether an error should be returned instead of calling the
//     real operation fn, decided at the configured Rate;
//   - err is the error to return when inject is true.
//
// resource is accepted for the per-resource rules extension; the global MVP
// rule applies uniformly regardless of resource.
func (in *Injector) maybe(resource string) (inject bool, sleep time.Duration, err error) {
	c := in.Config()
	if !c.Enabled {
		return false, 0, nil
	}

	// Safety guardrails: auto-off after MaxDuration since the first affected
	// call, or after MaxAffected affected calls. Lets an operator "set fire and
	// walk away" — a forgotten fault self-heals rather than running until the
	// next deploy. Zero (the default) disables each bound; both gate ALL fault
	// effects (latency + error). firstAt is stamped on the first affected call,
	// so the MaxDuration window is measured from then, not from process start.
	if c.MaxDuration > 0 {
		if first := in.firstAt.Load(); first != 0 && time.Since(time.Unix(0, first)) > c.MaxDuration {
			return false, 0, nil
		}
	}
	if c.MaxAffected > 0 && in.affected.Load() >= c.MaxAffected {
		return false, 0, nil
	}

	// Effective knobs: a matching per-resource Rule overrides the global; the
	// global is the fallback when no rule matches (so Rules add specificity
	// without losing the default). resource "" never matches a specific rule,
	// only a catch-all — matching the historic "uniform" behavior.
	rate, latency, errKind := c.Rate, c.Latency, c.Error
	if r, ok := matchRule(c.Rules, resource); ok {
		rate, latency, errKind = r.Rate, r.Latency, r.Error
	}

	if latency > 0 {
		sleep = latency
	}
	if rate > 0 && rand.Float64() < rate {
		inject = true
		err = mapError(errKind)
	}
	if sleep > 0 || inject {
		in.arm()
	}
	return inject, sleep, err
}

// matchRule returns the first rule in rules that matches resource (catch-all
// when a rule has no Resources), or (_, false) when none match.
func matchRule(rules []Rule, resource string) (Rule, bool) {
	for _, r := range rules {
		if r.matches(resource) {
			return r, true
		}
	}
	return Rule{}, false
}

// arm records that one call received a fault effect (latency and/or error):
// stamps firstAt on the first such call and counts every one, so the
// MaxDuration / MaxAffected guardrails have something to bound. The CAS makes
// the first-call stamp race-safe; the counter is a plain atomic add.
func (in *Injector) arm() {
	in.firstAt.CompareAndSwap(0, time.Now().UnixNano())
	in.affected.Add(1)
}
