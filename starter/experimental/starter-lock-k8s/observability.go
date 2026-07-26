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

package StarterLockK8s

import (
	"context"

	"go-spring.org/spring/experimental/cloud/lock"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
)

// tracerName identifies spans emitted by this starter.
const tracerName = "go-spring.org/starter-lock-k8s"

// wrapLockerBean is a bean constructor that creates the original k8s locker,
// then wraps it with OTel tracing.
func wrapLockerBean(c Config, inner lock.Locker) lock.Locker {
	return WrapLocker(inner)
}

// WrapLocker returns a lock.Locker that wraps Acquire and TryAcquire with OTel
// client spans.
func WrapLocker(inner lock.Locker) lock.Locker {
	return &observedLocker{inner: inner}
}

type observedLocker struct {
	inner lock.Locker
}

func (l *observedLocker) Acquire(ctx context.Context, key string, opts ...lock.Option) (lock.Lock, error) {
	ctx, span := otel.Tracer(tracerName).Start(ctx, "lock.acquire",
		trace.WithSpanKind(trace.SpanKindClient),
		trace.WithAttributes(
			attribute.String("lock.key", key),
			attribute.String("lock.system", "k8s"),
		),
	)
	held, err := l.inner.Acquire(ctx, key, opts...)
	if err != nil {
		span.SetStatus(codes.Error, err.Error())
		span.RecordError(err)
	}
	span.End()
	return held, err
}

func (l *observedLocker) TryAcquire(ctx context.Context, key string, opts ...lock.Option) (lock.Lock, bool, error) {
	ctx, span := otel.Tracer(tracerName).Start(ctx, "lock.try_acquire",
		trace.WithSpanKind(trace.SpanKindClient),
		trace.WithAttributes(
			attribute.String("lock.key", key),
			attribute.String("lock.system", "k8s"),
		),
	)
	held, ok, err := l.inner.TryAcquire(ctx, key, opts...)
	if err != nil {
		span.SetStatus(codes.Error, err.Error())
		span.RecordError(err)
	}
	if !ok {
		span.SetAttributes(attribute.Bool("lock.acquired", false))
	}
	span.End()
	return held, ok, err
}

func (l *observedLocker) Close() error {
	return l.inner.Close()
}
