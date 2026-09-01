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

package lock

import (
	"context"

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

// WrapLocker returns a [Locker] that wraps Acquire and TryAcquire with the
// observe kit's three signals (trace span + duration/in-flight metric +
// access log), labelled lock.system=system and lock.key=<key>. cfg controls
// the access log (off/brief/detailed). When starter-otel is not imported the
// global OTel providers are no-ops, so the wrapper adds negligible overhead
// and changes no behaviour.
//
// A lock starter installs it with its backend's system value and its
// per-instance observability config:
//
//	locker = lock.WrapLocker("redis", c.Observer.Observability, inner)
func WrapLocker(system string, cfg observe.ObserveConfig, inner Locker) Locker {
	return &observedLocker{
		obs:   observe.New(system, lockSemConv, trace.SpanKindClient, cfg),
		inner: inner,
	}
}

type observedLocker struct {
	obs   *observe.Observer
	inner Locker
}

func (l *observedLocker) Acquire(ctx context.Context, key string, opts ...Option) (Lock, error) {
	ctx, sp := l.obs.Start(ctx, "acquire", key)
	held, err := l.inner.Acquire(ctx, key, opts...)
	sp.End(err)
	return held, err
}

func (l *observedLocker) TryAcquire(ctx context.Context, key string, opts ...Option) (Lock, bool, error) {
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
