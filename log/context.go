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

// This file holds the two ways a log field travels with a context: [WithFields]
// pushes fields downward to everything below the frame that attached them, and
// [Collector] gathers fields upward so the frame that started the request can
// print them at its end. [contextFields] fixes the order in which the two reach
// a log event.

// -------------------------------------------------------------------------- //
// Fields carried downward: the WithFields chain.
// -------------------------------------------------------------------------- //

// carrierKey is the private key under which a context carries log fields. An
// unexported empty-struct type keeps it collision-free: no other package can
// produce a value that compares equal to it.
type carrierKey struct{}

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
	// The stored slice is shared with sibling contexts derived from the same
	// parent, so build a fresh one instead of appending in place.
	prev := CarriedFields(ctx)
	next := make([]Field, 0, len(prev)+len(fields))
	next = append(next, prev...)
	next = append(next, fields...)
	return context.WithValue(ctx, carrierKey{}, next)
}

// RootFields returns a fresh root context carrying exactly fields -- the head of
// a path, with nothing to inherit. It is [WithFields] rooted at
// [context.Background] rather than at an existing context, for the frame that
// mints a path instead of extending one:
//
//	ctx := log.RootFields(log.String("trace_id", id), log.String("data_id", d))
//
// Nothing upstream is carried, so fields cannot accumulate across calls the way
// they do down a derivation chain. With no fields it is [context.Background].
func RootFields(fields ...Field) context.Context {
	return WithFields(context.Background(), fields...)
}

// CarriedFields returns the fields the [WithFields] chain put on the context,
// outermost source first, or nil when it carries none (a nil context carries
// none either).
//
// It is the read side of [WithFields]: a frame that cannot derive from the
// source context -- a long-lived context that must borrow a request's fields,
// say -- re-roots them onto one of its own:
//
//	ctx = log.WithFields(appCtx, log.CarriedFields(reqCtx)...)
//
// Only the WithFields chain is returned. What a [Collector] has gathered is
// bound to the request that installed it and is not carried across; what a
// user-installed [FieldsFromContext] would contribute is not stored on the
// context at all.
func CarriedFields(ctx context.Context) []Field {
	if ctx == nil {
		return nil
	}
	if c, ok := ctx.Value(carrierKey{}).([]Field); ok {
		return c
	}
	return nil
}

// -------------------------------------------------------------------------- //
// Fields gathered upward: the Collector.
// -------------------------------------------------------------------------- //

// collectorKey is the private key under which a context carries a [Collector].
// Like [carrierKey] it is an unexported empty struct, so only this package can
// address the slot.
type collectorKey struct{}

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
	// mu guards fields; goroutines spawned under the same request may call
	// [Collect] concurrently.
	mu sync.Mutex
	// fields holds the accumulated fields in append order, and is only ever
	// appended to.
	fields []Field
}

// NewCollector returns a context carrying a fresh [Collector], together with
// that Collector. Handlers accumulate with [Collect]; the frame that installed
// it reads the result back with [Collector.Fields] when the request ends.
//
// Installing one is explicit. With no Collector on the context, [Collect] does
// nothing rather than guessing where the fields should have gone.
//
// ctx must not be nil, unlike in [Collect]. Installing a second Collector over
// an already-equipped context shadows the first rather than merging with it:
// context lookup stops at the nearest key, so [Collect] reaches only the
// innermost one.
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

// collectedFields returns a snapshot of the fields the context's [Collector] has
// accumulated, or nil when it carries none.
func collectedFields(ctx context.Context) []Field {
	if c, ok := ctx.Value(collectorKey{}).(*Collector); ok {
		return c.Fields()
	}
	return nil
}

// contextFields returns the fields a context itself carries, in evaluation
// order: the [WithFields] chain first (outermost source first, so the innermost
// wins among them), then the [Collector]'s collected fields. The result is
// always a fresh slice the caller may append to.
func contextFields(ctx context.Context) []Field {
	if ctx == nil {
		return nil
	}
	carried := CarriedFields(ctx)
	collected := collectedFields(ctx)
	if len(carried) == 0 && len(collected) == 0 {
		return nil
	}
	out := make([]Field, 0, len(carried)+len(collected))
	out = append(out, carried...)
	out = append(out, collected...)
	return out
}
