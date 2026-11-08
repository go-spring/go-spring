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
//   - bulkhead.usage_ratio and bulkhead.queued are the slot pool's saturation
//     and the callers currently blocked waiting for a slot.
//
// Every series carries system and service, the guard's identity.
package admission

import (
	"context"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"

	"go-spring.org/cloud/observability/instrument"
	"go-spring.org/stdlib/errutil"
)

// stateInstrumentSet is the inbound state plane's instruments: one set per
// manager (or per standalone guard), and the manager is one per process. It
// holds no per-guard state.
type stateInstrumentSet struct {
	meter      metric.Meter
	bulkUsage  metric.Float64ObservableGauge
	bulkQueued metric.Int64ObservableGauge
}

// buildStateInstruments resolves this package's state instruments against the
// meter provider the otel starter installs before any bean is constructed. The
// owner hands the set to every guard it builds, so a failed registration fails
// construction: a declared metric family that is not a legal instrument name
// would otherwise silently delete the whole state surface.
func buildStateInstruments() (*stateInstrumentSet, error) {
	m := otel.Meter(instrument.Scope)
	in := &stateInstrumentSet{meter: m}
	var err error
	in.bulkUsage, err = m.Float64ObservableGauge("resilience.server.bulkhead.usage_ratio",
		metric.WithDescription("Bulkhead slots held / capacity"),
		metric.WithUnit("1"))
	if err != nil {
		return nil, errutil.Explain(err, "admission: register resilience.server.bulkhead.usage_ratio")
	}

	in.bulkQueued, err = m.Int64ObservableGauge("resilience.server.bulkhead.queued",
		metric.WithDescription("Callers blocked waiting for a bulkhead slot"),
		metric.WithUnit("{caller}"))
	if err != nil {
		return nil, errutil.Explain(err, "admission: register resilience.server.bulkhead.queued")
	}
	return in, nil
}

// readyObserve registers this guard's state callback on the instrument set its
// owner resolved at construction. It runs on the first Admit, not at
// construction: the callback must exist before the first collection, and a
// guard that never admits never registers one.
//
// Registration and the closed check share s.mu so a Close racing the first
// admit cannot leave a callback registered.
func (s *server) readyObserve() {
	s.observeOnce.Do(func() {
		in := s.ins
		s.mu.Lock()
		defer s.mu.Unlock()
		if s.closed {
			return
		}
		reg, err := in.meter.RegisterCallback(s.observeState,
			in.bulkUsage, in.bulkQueued)
		if err != nil {
			instrument.ReportInstrumentError("resilience.server.bulkhead.usage_ratio", err)
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
	in := s.ins

	s.mu.Lock()
	st := s.st
	s.mu.Unlock()
	if st == nil {
		return nil
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
