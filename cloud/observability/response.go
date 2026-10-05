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

// response.go carries the half of a SERVER operation's identity that only exists
// once the handler has answered.
//
// A client declares its whole operation up front — it knows the destination and
// the verb before it calls. A server cannot: the response status code is the
// outcome of the work, so it is unknown when the declaration is read at
// [WithOperation] time. This file is the holder that closes that gap: the
// emitter installs one, the handler fills it on the way out, the emitter reads
// it back and attaches what it finds to the span and the access log.
//
// It is the inbound counterpart of the client-side [Recorder], and deliberately
// its mirror in shape: a mutable carrier the emitter owns and reads, never a
// signal the handler emits.
//
// What a handler records here is the LATE HALF OF THE OPERATION'S BOUNDED
// ATTRIBUTES — the same kind of value [Operation.Attrs] carries, just not known
// in time to be declared. A response status code is the example: bounded, and
// worth a metric label. So these reach all three signals, exactly as Attrs do.
// Per-call data that is unbounded in principle still belongs in
// [Operation.Detail], which is declared up front and never labelled.

package observability

import (
	"context"
	"sync"

	"go.opentelemetry.io/otel/attribute"
)

// responseKey is the private key under which a context carries the response
// holder. An unexported empty-struct type keeps it collision-free.
type responseKey struct{}

// Response is where a server handler records what only it knows once it has
// answered — in practice the response status code. A nil *Response is valid and
// discards every write, so a handler may call [ResponseFrom] and use the result
// without checking: a call that runs outside a server emitter is simply not
// recorded, rather than panicking on the path that is least tested.
type Response struct {
	mu    sync.Mutex
	attrs []attribute.KeyValue
}

// WithResponse returns ctx together with the holder a handler fills as it
// answers. The emitter installs it; nothing else should need to.
func WithResponse(ctx context.Context) (context.Context, *Response) {
	r := &Response{}
	return context.WithValue(ctx, responseKey{}, r), r
}

// ResponseFrom returns the holder ctx carries, or nil when it carries none —
// which is the ordinary case for a call that is not an inbound request.
func ResponseFrom(ctx context.Context) *Response {
	r, _ := ctx.Value(responseKey{}).(*Response)
	return r
}

// Add records attributes the handler learned while answering. It is nil-safe.
//
// Later values for the same key win, so a handler that writes the same key twice
// (a redirect chain, say) leaves the last answer standing — the same rule
// [WithSpanAttributes] applies.
func (r *Response) Add(attrs ...attribute.KeyValue) {
	if r == nil || len(attrs) == 0 {
		return
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	r.attrs = append(r.attrs, attrs...)
}

// Attributes returns a copy of what the handler recorded, in the order it
// recorded it.
func (r *Response) Attributes() []attribute.KeyValue {
	if r == nil {
		return nil
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if len(r.attrs) == 0 {
		return nil
	}
	out := make([]attribute.KeyValue, len(r.attrs))
	copy(out, r.attrs)
	return out
}
