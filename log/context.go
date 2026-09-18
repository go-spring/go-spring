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

package log

import (
	"context"
	"sync"
)

// carrierKey is the private key under which a context carries log fields. An
// unexported empty-struct type keeps it collision-free: no other package can
// produce a value that compares equal to it.
type carrierKey struct{}

// collectorKey is the private key under which a context carries a [Collector].
type collectorKey struct{}

// carrier is what [WithFields] stores on a context: the fields to attach to
// every event printed with that context. Each call stores a fresh slice, so
// sibling contexts derived from one parent never observe each other's fields.
type carrier []Field

// WithFields returns a context carrying fields that every log event printed
// with it will include, on top of whatever the context already carried. Fields
// accumulate down the derivation chain; when two sources supply the same key,
// the one evaluated later wins -- see [contextFields] for the full order,
// including where a user-installed [FieldsFromContext] sits in it.
//
// Fields are snapshotted here rather than read lazily. A value that only
// becomes known later belongs either on a context derived at that later point,
// or in a [Collector], which accumulates over the request and is read at its
// end.
//
// The fields live exactly as long as the context does. There is no Clear
// counterpart, and none is needed: a context that leaves scope takes its fields
// with it, and unlike a thread-local, a context is never pooled for reuse.
//
// WithFields may be called from any frame, including business code: the fields
// it adds reach every log event below that frame, framework-emitted ones
// included.
func WithFields(ctx context.Context, fields ...Field) context.Context {
	if len(fields) == 0 {
		return ctx
	}
	prev := carriedFields(ctx)
	next := make([]Field, 0, len(prev)+len(fields))
	next = append(next, prev...)
	next = append(next, fields...)
	return context.WithValue(ctx, carrierKey{}, carrier(next))
}

// carriedFields returns the fields the context carries, or nil.
func carriedFields(ctx context.Context) []Field {
	if c, ok := ctx.Value(carrierKey{}).(carrier); ok {
		return c
	}
	return nil
}

// Collector accumulates fields produced while a request is handled, so the
// frame that started the request can print them once at its end -- the
// wide-event / canonical-log-line shape.
//
// It exists because [WithFields] cannot serve that shape: those fields flow
// downward and are visible only below the frame that added them, while a
// middleware holds the context it created and can never see what a deeper frame
// added. A Collector is mutable and shared, so it can be written from anywhere
// in the call tree and read at the top.
//
// A Collector is not an emitter. When to log, at which level, and whether to
// sample are the caller's decisions; this only collects.
type Collector struct {
	mu     sync.Mutex
	fields []Field
}

// NewCollector returns a context carrying a fresh [Collector], together with
// that Collector. Handlers accumulate with [Collect]; the frame that installed
// it reads the result back with [Collector.Fields] when the request ends.
//
// Installing one is explicit. With no Collector on the context, [Collect] does
// nothing rather than guessing where the fields should have gone.
func NewCollector(ctx context.Context) (context.Context, *Collector) {
	c := &Collector{}
	return context.WithValue(ctx, collectorKey{}, c), c
}

// Collect appends fields to the [Collector] carried by ctx. It is a no-op when
// the context is nil or carries no Collector (see [NewCollector]), matching the
// rest of go-spring's optional hooks, which stay silent rather than break a
// request over instrumentation.
func Collect(ctx context.Context, fields ...Field) {
	if ctx == nil || len(fields) == 0 {
		return
	}
	c, ok := ctx.Value(collectorKey{}).(*Collector)
	if !ok {
		return
	}
	c.mu.Lock()
	c.fields = append(c.fields, fields...)
	c.mu.Unlock()
}

// Fields returns a snapshot of the fields collected so far, safe to read while
// other goroutines are still calling [Collect].
func (c *Collector) Fields() []Field {
	c.mu.Lock()
	defer c.mu.Unlock()
	return append([]Field(nil), c.fields...)
}

// contextFields returns the fields a context itself carries, in evaluation
// order: the [WithFields] chain first (outermost source first, so the innermost
// wins among them), then the [Collector]'s collected fields. The result is
// always a fresh slice the caller may append to.
func contextFields(ctx context.Context) []Field {
	if ctx == nil {
		return nil
	}
	carried := carriedFields(ctx)
	var collected []Field
	if c, ok := ctx.Value(collectorKey{}).(*Collector); ok {
		collected = c.Fields()
	}
	if len(carried) == 0 && len(collected) == 0 {
		return nil
	}
	out := make([]Field, 0, len(carried)+len(collected))
	out = append(out, carried...)
	out = append(out, collected...)
	return out
}
