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
	"context"
	"math/rand/v2"
	"sync/atomic"
	"time"

	"go-spring.org/cloud/traffic"
	"go-spring.org/stdlib/timeutil"
)

// side is one direction's live fault state: the config it reads and the
// guardrails it counts. Client and server each own one, so a fire on one side
// can neither read nor trip the other — separate configs, and separate
// MaxDuration/MaxAffected counters, so a client fire that self-heals cannot
// disarm a server one and a blast-radius cap on one side cannot silence the
// other.
//
// Both directions run the same [Config] type through the same injection
// sequence ([Injector.gate]): the fault model is direction-symmetric — a
// fraction of calls fails or slows down — so only the values differ, never the
// shape.
type side struct {
	cfg      atomic.Pointer[Config]
	firstAt  atomic.Int64 // unix-nanos of the first affected call; 0 = none yet
	affected atomic.Int64 // count of calls that received any fault effect
}

// config returns a snapshot of this side's live config (the zero Config if none
// has been set, which is a no-op).
func (s *side) config() Config {
	if c := s.cfg.Load(); c != nil {
		return *c
	}
	return Config{}
}

// Configs is the direction pair the governance center pushes — one [Config] per
// direction, mirroring spring.governance.client.fault and spring.governance.server.fault. Holding them
// as a pair keeps a single push from being able to swap the two by accident, the
// way two positional arguments of a setter could.
type Configs struct {
	// Client is the outbound side's config, read by [WrapClientExecutor]'s per-attempt
	// gate.
	Client Config

	// Server is the inbound side's config, read by [ApplyServer].
	Server Config
}

// Injector holds one [side] per traffic direction behind atomic pointers, so the
// center can hot-swap them from a source push without taking a lock on the
// execute path. The load-test propagator is shared, because the load-test marker
// is a property of the call, not of the direction.
//
// A side's zero config is inert (see [Config.Inert]) and injects nothing, so an
// Injector built from zero [Configs] is a transparent no-op in both directions.
type Injector struct {
	client side
	server side
	prop   traffic.Propagator // fixed at construction; read on every gated call
}

// NewInjector returns an Injector holding c, one config per direction. Either
// side can be replaced later with [Injector.SetConfig].
//
// prop is the load-test convention this injector asks when a rule's Scope
// distinguishes synthetic from real traffic (required — the starter injects
// the propagator bean).
func NewInjector(c Configs, prop traffic.Propagator) *Injector {
	in := &Injector{prop: prop}
	in.SetConfig(c)
	return in
}

// SetConfig atomically swaps BOTH directions' live config. It is safe to call
// concurrently with a gated call; each call observes the config pointer current
// at its own [side.maybe] read, so a toggle takes effect on the next operation.
//
// The two stores are not one transaction, and deliberately so: the sides are
// independent by design, and no call ever reads both — an outbound attempt reads
// only the client side, an inbound request only the server side — so there is no
// reader that could observe a torn pair.
func (in *Injector) SetConfig(c Configs) {
	in.client.cfg.Store(&c.Client)
	in.server.cfg.Store(&c.Server)
}

// ClientConfig returns a snapshot of the outbound side's live config (the zero
// Config if none has been set, which is a no-op).
func (in *Injector) ClientConfig() Config { return in.client.config() }

// ServerConfig returns a snapshot of the inbound side's live config.
func (in *Injector) ServerConfig() Config { return in.server.config() }

// maybe decides the fault applied to one call of this side, given its
// service, from one snapshot of the side's live config (a push landing
// mid-call cannot half-apply):
//   - sleep is a latency to apply first (always, when the effective Latency > 0);
//   - inject reports whether an error should be returned instead of calling the
//     real operation fn, decided at the effective Rate;
//   - err is the error to return when inject is true.
//
// The effective Rate/Latency/Error come from the first rule matching service
// (catch-all when a rule's Service is empty), falling back to the global
// values; see the inline comments for the guardrails that gate everything.
// Latency and error are independent knobs; a call that gets either counts as
// affected for the guardrails.
func (s *side) maybe(service string) (inject bool, sleep time.Duration, err error) {
	c := s.config()
	if c.Inert() {
		return false, 0, nil
	}

	// Safety guardrails: auto-off after MaxDuration since the first affected
	// call, or after MaxAffected affected calls. Lets an operator "set fire and
	// walk away" — a forgotten fault self-heals rather than running until the
	// next deploy. Zero (the default) disables each bound; both gate ALL fault
	// effects (latency + error). firstAt is stamped on the first affected call,
	// so the MaxDuration window is measured from then, not from process start.
	// The counters live on the side, so each direction self-heals on its own.
	if c.MaxDuration > 0 {
		if first := s.firstAt.Load(); first != 0 && time.Since(time.Unix(0, first)) > c.MaxDuration {
			return false, 0, nil
		}
	}
	if c.MaxAffected > 0 && s.affected.Load() >= c.MaxAffected {
		return false, 0, nil
	}

	// Effective knobs: a matching per-service Rule overrides the global; the
	// global is the fallback when no rule matches (so Rules add specificity
	// without losing the default). service "" never matches a specific rule,
	// only a catch-all — matching the historic "uniform" behavior.
	rate, latency, jitter, errKind := c.Rate, c.Latency, c.LatencyJitter, c.Error
	if r, ok := matchRule(c.Rules, service); ok {
		rate, latency, jitter, errKind = r.Rate, r.Latency, r.LatencyJitter, r.Error
	}

	if latency > 0 {
		sleep = latency
		if jitter > 0 {
			// Uniform draw in [-jitter, +jitter], clamped at 0: latency stays a
			// lower bound, and a jitter larger than latency cannot make it
			// negative. Jitter breaks the resonance of a fixed sleep with a
			// fixed retry backoff.
			sleep += time.Duration((rand.Float64()*2 - 1) * float64(jitter))
			if sleep < 0 {
				sleep = 0
			}
		}
	}
	if rate > 0 && rand.Float64() < rate {
		inject = true
		err = newInjectedError(errKind)
	}
	if sleep > 0 || inject {
		// Record the effect for the guardrails above: stamp the first affected
		// call (the CAS makes the stamp race-safe) and count every one.
		s.firstAt.CompareAndSwap(0, time.Now().UnixNano())
		s.affected.Add(1)
	}
	return inject, sleep, err
}

// matchRule returns the first rule in rules that matches service (catch-all
// when a rule has an empty Service), or (_, false) when none match.
func matchRule(rules []Rule, service string) (Rule, bool) {
	for _, r := range rules {
		if r.matches(service) {
			return r, true
		}
	}
	return Rule{}, false
}

// scopeApplies reports whether fault injection should run for a call, given the
// live config's Scope and the call's load-test marker. Scope "" (or unknown)
// injects into all traffic; "real" skips load-test calls; "loadtest" skips real
// calls. It is the shared gate used by both the client-side [WrapClientExecutor] and
// the server-side [ApplyServer], so the two directions honour the same scoping rule.
func scopeApplies(c Config, prop traffic.Propagator, ctx context.Context) bool {
	switch c.Scope {
	case "real":
		return !prop.IsLoadTest(ctx)
	case "loadtest":
		return prop.IsLoadTest(ctx)
	default:
		return true
	}
}

// gate gates fn with one side's fault rules, applying the configured latency
// before fn and returning the injected error instead of calling fn when the rate
// hits. It is the ONE shared fault sequence — both directions run through it (the
// client-side [WrapClientExecutor] per-attempt closure with the client side, the
// server-side [ApplyServer] with the server side) — so scope gating, guardrails and
// cancellation semantics cannot drift apart.
//
// The side is a parameter rather than a receiver so the DIRECTION is fixed by the
// caller inside this package: [WrapClientExecutor] and [ApplyServer] each name their own
// side, and no caller outside can aim a gate at the wrong half of the process.
//
// fn receives the caller's (or the executor's per-attempt) context so
// cancellation propagates into the real operation. The latency sleep is
// cancellable via ctx; on cancel the context error is returned so the
// caller's own timeout/budget logic reacts rather than retrying blindly.
// gate does NOT retry — retry is a client concern; for the full
// retry/timeout/breaker treatment on a client call use [WrapClientExecutor] with a
// chain.Executor.
func (in *Injector) gate(ctx context.Context, s *side, service string, fn func(context.Context) error) error {
	if c := s.config(); !scopeApplies(c, in.prop, ctx) {
		return fn(ctx)
	}
	inject, sleep, injErr := s.maybe(service)
	if sleep > 0 && !timeutil.Sleep(ctx, sleep) {
		return ctx.Err()
	}
	if inject {
		return injErr
	}
	return fn(ctx)
}
