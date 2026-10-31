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

// observe.go is the executor's OWN state plane. The organs this executor
// composes — the circuit breaker and the rate budget — are observed here, by
// the package that owns them, and nowhere else: the request-plane observer in
// .../outbound/observe records what happened to one call, never what state a
// governance organ is in.
//
// State is pulled, not pushed. Each instrument is an observable one, and one
// callback per executor reports the current state at collection time:
//
//   - breaker.state is 1-hot: one series per state, the current one at 1.
//   - breaker.transitions_total counts entries into each state. It is an
//     observable counter, so a transition shorter than a collection interval is
//     still counted (a gauge would miss a whole open interval between scrapes).
//     Together, state and transitions answer "is it cut off now" and "how often"
//     — start and end times are the gauge's edges in the time series.
//   - ratelimit.usage_ratio is the local budget's spent fraction. It is absent
//     when a ClusterLimiter is configured, because then the local budget is
//     never charged and would read zero. It counts every charge on the
//     governance path, including attempts the breaker later refused and retried
//     attempts.
//
// The attributes carry service and unit (the policy key), but deliberately not
// system: one executor serves every client backend of a label, so it does not
// know the system; system stays a request-plane dimension.
package resilience

import (
	"context"
	"sync"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"

	"go-spring.org/cloud/governance/capability/circuitbreaker"
	"go-spring.org/cloud/governance/capability/ratelimit"
	"go-spring.org/cloud/observability/instrument"
)

// stateInstrumentSet is the outbound state plane's instruments: one set per
// process, resolved lazily so it binds to whichever meter provider is current
// at first use. It holds no per-executor state.
type stateInstrumentSet struct {
	meter       metric.Meter
	state       metric.Float64ObservableGauge
	transitions metric.Int64ObservableCounter
	rateUsage   metric.Float64ObservableGauge
}

// stateInstruments is the one state instrument set this package uses.
var stateInstruments = sync.OnceValue(buildStateInstruments)

func buildStateInstruments() *stateInstrumentSet {
	m := otel.Meter(instrument.Scope)
	in := &stateInstrumentSet{meter: m}
	var err error
	if in.state, err = m.Float64ObservableGauge("resilience.client.breaker.state",
		metric.WithDescription("Circuit-breaker state, 1-hot by state")); err != nil {
		instrument.ReportInstrumentError("resilience.client.breaker.state", err)
	}
	if in.transitions, err = m.Int64ObservableCounter("resilience.client.breaker.transitions_total",
		metric.WithDescription("Circuit-breaker transitions, by destination state"),
		metric.WithUnit("{transition}")); err != nil {
		instrument.ReportInstrumentError("resilience.client.breaker.transitions_total", err)
	}
	if in.rateUsage, err = m.Float64ObservableGauge("resilience.client.ratelimit.usage_ratio",
		metric.WithDescription("Local rate budget spent / current ceiling (includes refused and retried attempts)"),
		metric.WithUnit("1")); err != nil {
		instrument.ReportInstrumentError("resilience.client.ratelimit.usage_ratio", err)
	}
	return in
}

// resetInstruments makes the next use of stateInstruments resolve a fresh set.
// It exists for tests that install their own MeterProvider: the set is
// process-wide and resolved once, so a test running after one that already
// resolved it would otherwise keep reporting into the earlier provider.
func resetInstruments() { stateInstruments = sync.OnceValue(buildStateInstruments) }

// readyObserve resolves the shared instruments and registers this executor's
// state callback. It runs on the first call, not at construction: the callback
// must exist before the first collection, and the instruments must be resolved
// after the global providers are installed — which may happen after the
// executor is built.
//
// Registration and the closed check share e.obsMu so a Close racing the first
// call cannot leave a callback registered: whichever gets the lock first wins,
// and a Close that lands first makes ready a no-op.
func (e *defaultExecutor) readyObserve() {
	e.observeOnce.Do(func() {
		in := stateInstruments()
		e.obsMu.Lock()
		defer e.obsMu.Unlock()
		if e.closed {
			return
		}
		reg, err := in.meter.RegisterCallback(e.observeState,
			in.state, in.transitions, in.rateUsage)
		if err != nil {
			instrument.ReportInstrumentError("resilience.client.breaker.state", err)
			return
		}
		e.observeReg = reg
	})
}

// closeObserve unregisters this executor's state callback. It is idempotent and
// is called by [defaultExecutor.Close]; the executor itself stays usable, but
// it publishes no state afterwards.
func (e *defaultExecutor) closeObserve() {
	e.mu.Lock()
	e.closed = true
	reg := e.observeReg
	e.observeReg = nil
	e.mu.Unlock()
	if reg != nil {
		_ = reg.Unregister()
	}
}

// breakerRef and budgetRef are the state a collection reads, copied out first
// so the callback never holds any executor state while touching an organ (or
// the OTel SDK).
type breakerRef struct {
	unit string
	b    *circuitbreaker.Breaker
}

type budgetRef struct {
	unit string
	b    ratelimit.Budget
}

// observeState reports every breaker and rate budget of the executor's current
// generation. A generation swap replaces cur wholesale; the copied pointers
// stay valid for the collection, so an in-flight reading never mixes two
// generations beyond the one instant of the copy. It takes no lock: the
// generation is loaded atomically and its key table is a sync.Map.
func (e *defaultExecutor) observeState(_ context.Context, o metric.Observer) error {
	in := stateInstruments()

	var breakers []breakerRef
	var budgets []budgetRef
	if st := e.breaker.Load(); st != nil {
		st.keys.Range(func(k, v any) bool {
			unit, cell := k.(string), v.(*breakerCell)
			if cell.b != nil { // the OFF stage publishes no state
				breakers = append(breakers, breakerRef{unit: unit, b: cell.b})
			}
			return true
		})
	}
	if st := e.rate.Load(); st != nil {
		st.keys.Range(func(k, v any) bool {
			budgets = append(budgets, budgetRef{unit: k.(string), b: v.(ratelimit.Budget)})
			return true
		})
	}
	cluster := e.limiter != nil

	for _, ref := range breakers {
		s := ref.b.Stats()
		for _, state := range [...]circuitbreaker.State{
			circuitbreaker.StateClosed,
			circuitbreaker.StateOpen,
			circuitbreaker.StateHalfOpen,
		} {
			value := 0.0
			if state == s.State {
				value = 1
			}
			o.ObserveFloat64(in.state, value, e.unitAttrs(ref.unit,
				attribute.String("state", state.String())))
		}
		o.ObserveInt64(in.transitions, int64(s.Trips), e.unitAttrs(ref.unit,
			attribute.String("to", circuitbreaker.StateOpen.String())))
		o.ObserveInt64(in.transitions, int64(s.HalfOpens), e.unitAttrs(ref.unit,
			attribute.String("to", circuitbreaker.StateHalfOpen.String())))
		o.ObserveInt64(in.transitions, int64(s.Recovers), e.unitAttrs(ref.unit,
			attribute.String("to", circuitbreaker.StateClosed.String())))
	}

	if cluster {
		// The store, not the local budget, is charged — a local reading would be
		// a permanent zero and actively misleading.
		return nil
	}
	for _, ref := range budgets {
		usage := ref.b.Usage()
		if usage.Limit <= 0 {
			continue
		}
		o.ObserveFloat64(in.rateUsage, usage.Used/usage.Limit, e.unitAttrs(ref.unit))
	}
	return nil
}

// unitAttrs is the identity of one protected unit: the executor's service label
// plus the policy key, with extra attributes appended.
func (e *defaultExecutor) unitAttrs(unit string, extra ...attribute.KeyValue) metric.MeasurementOption {
	attrs := make([]attribute.KeyValue, 0, 2+len(extra))
	attrs = append(attrs,
		attribute.String("service", e.service),
		attribute.String("unit", unit))
	attrs = append(attrs, extra...)
	return metric.WithAttributes(attrs...)
}
