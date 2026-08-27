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

// Package lockobserve is the shared distributed-lock instrumentation adapter
// for the go-spring observability story. It wraps any [lock.Locker] so Acquire
// and TryAcquire emit the observe kit's full three signals — trace span +
// duration/in-flight metric + access log — through [observe.New] with a lock
// semantic convention (lock.system / lock.key attributes, metrics under
// lock.operation.duration), instead of the trace-only wrapper this bridge used
// to be.
//
// It lives in the observe package (rather than copy-pasted into each lock
// starter, or inside the otel-free abstraction packages that define [lock.Locker]) so
// the four lock starters share one implementation instead of duplicating a
// ~70-line wrapper each, differing only in the system label. A starter installs
// it with its backend's system value and its per-instance observability config:
//
//	locker = lockobserve.WrapLocker("redis", c.Observer.Observability, inner)
package lockobserve

import (
	"context"

	"go-spring.org/cloud/experimental/lock"
	observe "go-spring.org/cloud/observe"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
)

// lockSemConv is the lock namespace: metrics under lock.*, attributes
// lock.system / lock.operation with the key captured as lock.key (in detailed
// log mode and as a span attribute).
var lockSemConv = observe.SemConv{
	Domain:    "lock",
	SystemKey: "lock.system",
	OpKey:     "lock.operation",
	ArgKey:    "lock.key",
}

// WrapLocker returns a [lock.Locker] that wraps Acquire and TryAcquire with the
// observe kit's three signals, labelled lock.system=system and lock.key=<key>.
// cfg controls the access log (off/brief/detailed). When starter-otel is not
// imported the global OTel providers are no-ops, so the wrapper adds negligible
// overhead and changes no behaviour.
func WrapLocker(system string, cfg observe.ObserveConfig, inner lock.Locker) lock.Locker {
	return &observedLocker{
		obs:   observe.New(system, lockSemConv, trace.SpanKindClient, cfg),
		inner: inner,
	}
}

type observedLocker struct {
	obs   *observe.Observer
	inner lock.Locker
}

func (l *observedLocker) Acquire(ctx context.Context, key string, opts ...lock.Option) (lock.Lock, error) {
	ctx, sp := l.obs.Start(ctx, "acquire", key)
	held, err := l.inner.Acquire(ctx, key, opts...)
	sp.End(err)
	return held, err
}

func (l *observedLocker) TryAcquire(ctx context.Context, key string, opts ...lock.Option) (lock.Lock, bool, error) {
	ctx, sp := l.obs.Start(ctx, "try_acquire", key)
	held, ok, err := l.inner.TryAcquire(ctx, key, opts...)
	var attrs []attribute.KeyValue
	if !ok {
		// A missed TryAcquire is not an error (err stays nil): record the miss
		// explicitly so the span/metric dimension separates it from a win.
		attrs = append(attrs, attribute.Bool("lock.acquired", false))
	}
	sp.End(err, attrs...)
	return held, ok, err
}

func (l *observedLocker) Close() error { return l.inner.Close() }
