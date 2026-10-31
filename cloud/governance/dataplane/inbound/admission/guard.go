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

package admission

import (
	"context"
	"time"

	kfault "go-spring.org/cloud/governance/capability/fault"
	"go-spring.org/cloud/governance/dataplane/inbound/chain"
)

// Identity names the server instance a [Guard] protects. It is bound ONCE, at
// construction, because every piece of state behind the Guard belongs to that
// instance: the one rate budget, the one bulkhead and the one config generation.
//
// System is the protocol family's own value — "gin", "grpc", "dubbo" — and it
// is what a signal labels itself with. Service is the instance label a policy
// rule matches, typically the listening address ("gin:0.0.0.0:8080"). The two
// are separate coordinates: several instances of one family are told apart by
// Service alone.
//
// The interface being invoked (the HTTP route, the RPC method) is NOT part of
// the identity: it varies per request and lives on [chain.Request.Action].
type Identity struct {
	// System is the server family's value ("gin", "grpc", "dubbo").
	System string

	// Service is this instance's label — the value a policy rule matches.
	Service string
}

// Verdict is why a request was admitted or refused. It is one word, not a
// protocol status: the adapter maps it onto its own wire representation.
type Verdict uint8

const (
	// VerdictAdmitted means every stage allowed the request. The handler is the
	// caller's to run.
	VerdictAdmitted Verdict = iota

	// VerdictRateLimited means the instance's rate budget was exhausted.
	VerdictRateLimited

	// VerdictBulkheadFull means the instance already handles the maximum number
	// of concurrent requests.
	VerdictBulkheadFull

	// VerdictFaultInjected means an injected fault produced this request's
	// failure. See [Result.Err].
	VerdictFaultInjected
)

// String returns the verdict's lowercase name for logs and labels.
func (v Verdict) String() string {
	switch v {
	case VerdictRateLimited:
		return "rate_limited"
	case VerdictBulkheadFull:
		return "bulkhead_full"
	case VerdictFaultInjected:
		return "fault_injected"
	default:
		return "admitted"
	}
}

// Result is one request's judgment: whether it may proceed, and the neutral
// facts behind that answer. It is returned by value and is read-only to the
// caller; its fields are unexported so it can be extended without a breaking
// change.
type Result struct {
	verdict    Verdict
	err        error
	retryAfter time.Duration
	latency    time.Duration
	timeout    time.Duration
	rule       string
	fault      kfault.Result
}

// Admitted reports whether the request may proceed to its handler.
func (r Result) Admitted() bool { return r.verdict == VerdictAdmitted }

// Verdict is why the request was admitted or refused.
func (r Result) Verdict() Verdict { return r.verdict }

// Err is the refusal's error, or the failure an injected fault produced. It is
// nil for an admitted request.
func (r Result) Err() error { return r.err }

// RetryAfter is how long the caller is asked to wait before retrying, when the
// refusing stage knows (a rate limit's max wait, a bulkhead's queue duration).
// Zero means unspecified.
func (r Result) RetryAfter() time.Duration { return r.retryAfter }

// Latency is the time the Guard itself spent before returning this result —
// injected fault delay included. It is NOT the handler's duration.
func (r Result) Latency() time.Duration { return r.latency }

// Rule names the ADMISSION rule that produced the verdict, for audit. Empty
// when no rule governed the request (the instance's default, or an inert
// configuration), and empty on an injected fault — the fault's own attribution
// is [Result.Fault], a different vocabulary that may name several experiments.
func (r Result) Rule() string { return r.rule }

// Timeout is the handling budget the policy asks for, or 0 for none. The Guard
// does not apply it — Done has no context to cancel — so the adapter wraps the
// handler's context with it. See [Policy.Timeout].
func (r Result) Timeout() time.Duration { return r.timeout }

// Fault is the fault stage's decision, zero when no fault was configured or the
// request was not affected. It is meaningful when Verdict is
// [VerdictFaultInjected], and also reports a delay-only injection.
func (r Result) Fault() kfault.Result { return r.fault }

// Done is the completion function returned alongside a [Result]. It is the
// request's other half: it returns the bulkhead slot. Call it exactly once —
// normally with defer — after the handler has finished. It is safe to call more
// than once. A configuration with no bulkhead returns a no-op Done, so callers
// never branch on "is protection on".
type Done func()

// Guard is the inbound governance object a server middleware holds. One
// instance per server instance, created at wiring time and captured by the
// middleware.
//
// It is safe for concurrent use. Implementations are supplied by this module's
// engine; adapters depend on this interface only.
type Guard interface {
	// Admit runs the pre-handler stages for one request and returns its result
	// and completion function. It never runs the handler.
	//
	// The returned Done is always non-nil: a refused request yields a no-op, so
	// an adapter can defer it unconditionally.
	Admit(ctx context.Context, req chain.Request) (Result, Done)

	// Close releases any background resources the Guard holds. It is safe to
	// call more than once.
	Close() error
}
