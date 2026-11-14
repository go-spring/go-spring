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

// observe.go is the inbound admission guard's state-plane surface: the
// bulkhead occupancy a guard publishes, the [Observer] seam, and the default
// OTel-backed adapter.

package admission

import (
	"context"
	"sync"

	"go-spring.org/cloud/observability/instrument"
	"go-spring.org/stdlib/errutil"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
)

// BulkheadState is one guard's current bulkhead occupancy.
type BulkheadState struct {
	// UsageRatio is slots held / capacity.
	UsageRatio float64
	// Queued is the number of callers blocked waiting for a slot.
	Queued int64
}

// Observer receives the admission layer's state-plane registrations. A nil
// Observer disables the state metrics; the guard then simply never registers a
// reader.
//
// The state is a pull, not a push: the adapter calls read on collection and must
// not block, so a reader must not take a lock that a request path holds while
// touching the metric backend.
type Observer interface {
	// ObserveBulkhead registers read as the source of one guard's occupancy,
	// labelled with system and service. The returned off removes it; after off
	// the reader is never called again.
	ObserveBulkhead(system, service string, read func() (BulkheadState, bool)) (off func())

	// Close releases the adapter's callback. It must be safe to call more than
	// once.
	Close() error
}

// NewOTelObserver resolves the state instruments and returns the default
// OTel-backed [Observer]. It is registered as the process's default bean.
func NewOTelObserver() (Observer, error) {
	m := otel.Meter(instrument.Scope)
	usage, err := m.Float64ObservableGauge("resilience.server.bulkhead.usage_ratio",
		metric.WithDescription("Bulkhead slots held / capacity"),
		metric.WithUnit("1"))
	if err != nil {
		return nil, errutil.Explain(err, "admission: register resilience.server.bulkhead.usage_ratio")
	}
	queued, err := m.Int64ObservableGauge("resilience.server.bulkhead.queued",
		metric.WithDescription("Callers blocked waiting for a bulkhead slot"),
		metric.WithUnit("{caller}"))
	if err != nil {
		return nil, errutil.Explain(err, "admission: register resilience.server.bulkhead.queued")
	}
	o := &otelObserver{
		usage:   usage,
		queued:  queued,
		readers: map[string]bulkheadReader{},
	}
	// One callback per adapter, not one per guard: a second creation-time
	// callback on the same instrument descriptor would be dropped.
	if reg, err := m.RegisterCallback(o.collect, usage, queued); err != nil {
		instrument.ReportInstrumentError("resilience.server.bulkhead.usage_ratio", err)
	} else {
		o.reg = reg
	}
	return o, nil
}

// bulkheadReader is one registered guard.
type bulkheadReader struct {
	system  string
	service string
	read    func() (BulkheadState, bool)
}

// otelObserver is the default [Observer].
type otelObserver struct {
	usage   metric.Float64ObservableGauge
	queued  metric.Int64ObservableGauge
	mu      sync.Mutex
	readers map[string]bulkheadReader
	reg     metric.Registration
}

// ObserveBulkhead implements [Observer].
func (o *otelObserver) ObserveBulkhead(system, service string, read func() (BulkheadState, bool)) func() {
	key := system + "|" + service
	o.mu.Lock()
	o.readers[key] = bulkheadReader{system: system, service: service, read: read}
	o.mu.Unlock()
	return func() {
		o.mu.Lock()
		delete(o.readers, key)
		o.mu.Unlock()
	}
}

// collect reports every registered guard's current occupancy. The reader list is
// snapshotted under the lock and read after it is released, so collection never
// holds this lock while touching a guard or the OTel SDK.
func (o *otelObserver) collect(_ context.Context, ob metric.Observer) error {
	o.mu.Lock()
	readers := make([]bulkheadReader, 0, len(o.readers))
	for _, r := range o.readers {
		readers = append(readers, r)
	}
	o.mu.Unlock()

	for _, r := range readers {
		state, ok := r.read()
		if !ok {
			continue
		}
		attrs := metric.WithAttributes(
			attribute.String("system", r.system),
			attribute.String("service", r.service),
		)
		ob.ObserveFloat64(o.usage, state.UsageRatio, attrs)
		ob.ObserveInt64(o.queued, state.Queued, attrs)
	}
	return nil
}

// Close implements [Observer].
func (o *otelObserver) Close() error {
	if o == nil {
		return nil
	}
	o.mu.Lock()
	reg := o.reg
	o.reg = nil
	o.readers = map[string]bulkheadReader{}
	o.mu.Unlock()
	if reg != nil {
		_ = reg.Unregister()
	}
	return nil
}

// readyObserve registers this guard's state reader on the observer its owner
// resolved at construction. It runs on the first Admit, not at construction: a
// guard that never admits never registers one.
//
// Registration and the closed check share s.mu so a Close racing the first
// admit cannot leave a reader registered.
func (s *server) readyObserve() {
	s.observeOnce.Do(func() {
		if s.obs == nil {
			return
		}
		s.mu.Lock()
		defer s.mu.Unlock()
		if s.closed {
			return
		}
		s.observeOff = s.obs.ObserveBulkhead(s.id.System, s.id.Service, s.bulkheadState)
	})
}

// closeObserve marks the guard closed and removes its state reader. It is
// idempotent and is shared by Close and the manager's own teardown.
func (s *server) closeObserve() {
	s.mu.Lock()
	s.closed = true
	off := s.observeOff
	s.observeOff = nil
	s.mu.Unlock()
	if off != nil {
		off()
	}
}

// bulkheadState snapshots this guard's current generation. The state pointer is
// copied under s.mu and read after it is released, so a generation swap replaces
// s.st wholesale and the copied pointer stays valid for this collection.
func (s *server) bulkheadState() (BulkheadState, bool) {
	s.mu.Lock()
	st := s.st
	s.mu.Unlock()
	if st == nil || st.bulk == nil {
		return BulkheadState{}, false
	}
	stats := st.bulk.Stats()
	return BulkheadState{
		UsageRatio: float64(stats.InUse) / float64(stats.Capacity),
		Queued:     int64(stats.Queued),
	}, true
}
