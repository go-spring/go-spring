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

// observe.go is the guard's OWN state plane. The stages this guard owns — the
// rate budget and the bulkhead — are observed here, by the package that owns
// them: the request-plane server observer in .../inbound/observe records what
// happened to one request, never what state a governance stage is in.
//
// State is pulled, not pushed. Each instrument is observable, and one callback
// per guard reports the current generation at collection time:
//
//   - ratelimit.usage_ratio is the local budget's spent fraction; it is absent
//     when a ClusterLimiter is configured or the stage is unlimited.
//   - bulkhead.usage_ratio and bulkhead.queued are the slot pool's saturation
//     and the callers currently blocked waiting for a slot.
//
// Every series carries system and service, the guard's identity.
package admission

import (
	"context"
	"sync"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"

	"go-spring.org/cloud/observability/instrument"
)

// stateInstrumentSet is the inbound state plane's instruments: one set per
// process, resolved lazily so it binds to whichever meter provider is current
// at first use. It holds no per-guard state.
type stateInstrumentSet struct {
	meter      metric.Meter
	rateUsage  metric.Float64ObservableGauge
	bulkUsage  metric.Float64ObservableGauge
	bulkQueued metric.Int64ObservableGauge
}

// stateInstruments is the one state instrument set this package uses.
var stateInstruments = sync.OnceValue(buildStateInstruments)

func buildStateInstruments() *stateInstrumentSet {
	m := otel.Meter(instrument.Scope)
	in := &stateInstrumentSet{meter: m}
	var err error
	if in.rateUsage, err = m.Float64ObservableGauge("resilience.server.ratelimit.usage_ratio",
		metric.WithDescription("Local rate budget spent / current ceiling"),
		metric.WithUnit("1")); err != nil {
		instrument.ReportInstrumentError("resilience.server.ratelimit.usage_ratio", err)
	}
	if in.bulkUsage, err = m.Float64ObservableGauge("resilience.server.bulkhead.usage_ratio",
		metric.WithDescription("Bulkhead slots held / capacity"),
		metric.WithUnit("1")); err != nil {
		instrument.ReportInstrumentError("resilience.server.bulkhead.usage_ratio", err)
	}
	if in.bulkQueued, err = m.Int64ObservableGauge("resilience.server.bulkhead.queued",
		metric.WithDescription("Callers blocked waiting for a bulkhead slot"),
		metric.WithUnit("{caller}")); err != nil {
		instrument.ReportInstrumentError("resilience.server.bulkhead.queued", err)
	}
	return in
}

// resetInstruments makes the next use of stateInstruments resolve a fresh set.
// It exists for tests that install their own MeterProvider.
func resetInstruments() { stateInstruments = sync.OnceValue(buildStateInstruments) }

// readyObserve resolves the shared instruments and registers this guard's state
// callback. It runs on the first Admit, not at construction: the callback must
// exist before the first collection, and the instruments must be resolved after
// the global providers are installed.
//
// Registration and the closed check share s.mu so a Close racing the first
// admit cannot leave a callback registered.
func (s *server) readyObserve() {
	s.observeOnce.Do(func() {
		in := stateInstruments()
		s.mu.Lock()
		defer s.mu.Unlock()
		if s.closed {
			return
		}
		reg, err := in.meter.RegisterCallback(s.observeState,
			in.rateUsage, in.bulkUsage, in.bulkQueued)
		if err != nil {
			instrument.ReportInstrumentError("resilience.server.ratelimit.usage_ratio", err)
			return
		}
		s.observeReg = reg
	})
}

// closeObserve marks the guard closed and unregisters its state callback. It is
// idempotent and is shared by Close and the manager's own teardown, so the two
// paths cannot diverge.
func (s *server) closeObserve() {
	s.mu.Lock()
	s.closed = true
	reg := s.observeReg
	s.observeReg = nil
	s.mu.Unlock()
	if reg != nil {
		_ = reg.Unregister()
	}
}

// observeState reports the guard's current generation. The state pointer is
// copied under s.mu and read after it is released, so the callback never holds
// the guard lock while touching an organ or the OTel SDK; a generation swap
// replaces s.st wholesale, and the copied pointer stays valid for this
// collection.
func (s *server) observeState(_ context.Context, o metric.Observer) error {
	in := stateInstruments()

	s.mu.Lock()
	st := s.st
	s.mu.Unlock()
	if st == nil {
		return nil
	}

	if usage := st.rate.Usage(); usage.Limit > 0 {
		o.ObserveFloat64(in.rateUsage, usage.Used/usage.Limit, s.attrs())
	}

	// A nil bulkhead is the OFF stage: no occupancy series to publish.
	if st.bulk != nil {
		stats := st.bulk.Stats()
		o.ObserveFloat64(in.bulkUsage, float64(stats.InUse)/float64(stats.Capacity), s.attrs())
		o.ObserveInt64(in.bulkQueued, int64(stats.Queued), s.attrs())
	}
	return nil
}

// attrs is the guard's identity prefix, with extra attributes appended.
func (s *server) attrs(extra ...attribute.KeyValue) metric.MeasurementOption {
	attrs := make([]attribute.KeyValue, 0, 2+len(extra))
	attrs = append(attrs,
		attribute.String("system", s.id.System),
		attribute.String("service", s.id.Service))
	attrs = append(attrs, extra...)
	return metric.WithAttributes(attrs...)
}
