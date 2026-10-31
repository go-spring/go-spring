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

// observe.go is the sentinel driver's state plane. The driver owns the
// breaker (sentinel-golang does), so the driver publishes its state: the mirror
// kept in breaker_listener.go is reported by one package-level observable
// callback, with the same metric names the builtin resilience engine uses, so a
// fleet sees one family whichever driver is configured.
//
// Registration is lazy and process-wide: sentinel's own listener is
// process-wide too, and a callback that reports an empty registry costs
// nothing, so there is no per-executor register/unregister race. An executor's
// Close removes its mirror; the callback then reports nothing for it.
package StarterGovernanceSentinel

import (
	"context"
	"sync"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"

	breaker "go-spring.org/cloud/governance/capability/circuitbreaker"
)

// scope is the meter scope the builtin client-side family uses, so sentinel's
// series land next to theirs.
const scope = "go-spring.org/cloud/governance/dataplane/resilience"

type stateInstrumentSet struct {
	meter       metric.Meter
	state       metric.Float64ObservableGauge
	transitions metric.Int64ObservableCounter
}

var stateInstruments = sync.OnceValue(buildStateInstruments)

// stateObserverOnce guards the one-time callback registration.
var stateObserverOnce sync.Once

func buildStateInstruments() *stateInstrumentSet {
	m := otel.Meter(scope)
	in := &stateInstrumentSet{meter: m}
	in.state, _ = m.Float64ObservableGauge("resilience.client.breaker.state",
		metric.WithDescription("Circuit-breaker state, 1-hot by state"))
	in.transitions, _ = m.Int64ObservableCounter("resilience.client.breaker.transitions_total",
		metric.WithDescription("Circuit-breaker transitions, by destination state"),
		metric.WithUnit("{transition}"))
	return in
}

// ensureStateObserver registers the process-wide state callback once. It is
// called on the first active breaker rule, after the global providers may have
// been installed.
func ensureStateObserver() {
	stateObserverOnce.Do(func() {
		in := stateInstruments()
		_, _ = in.meter.RegisterCallback(observeBreakers,
			in.state, in.transitions)
	})
}

// observeBreakers reports every live mirror. The mirror's own mutex is taken
// only inside stats(), and OTel is not touched while holding it.
func observeBreakers(_ context.Context, o metric.Observer) error {
	in := stateInstruments()
	breakerRoutes.Range(func(key, value any) bool {
		service, _ := key.(string)
		m, _ := value.(*breakerMirror)
		if m == nil {
			return true
		}
		stats := m.stats()
		for _, state := range [...]breaker.State{
			breaker.StateClosed,
			breaker.StateOpen,
			breaker.StateHalfOpen,
		} {
			value := 0.0
			if state == stats.State {
				value = 1
			}
			o.ObserveFloat64(in.state, value, metric.WithAttributes(
				attribute.String("service", service),
				attribute.String("state", state.String())))
		}
		o.ObserveInt64(in.transitions, int64(stats.Trips), metric.WithAttributes(
			attribute.String("service", service),
			attribute.String("to", breaker.StateOpen.String())))
		o.ObserveInt64(in.transitions, int64(stats.HalfOpens), metric.WithAttributes(
			attribute.String("service", service),
			attribute.String("to", breaker.StateHalfOpen.String())))
		o.ObserveInt64(in.transitions, int64(stats.Recovers), metric.WithAttributes(
			attribute.String("service", service),
			attribute.String("to", breaker.StateClosed.String())))
		return true
	})
	return nil
}
