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

package loadbalance

import (
	"context"
	"errors"

	"go-spring.org/cloud/governance/dataplane/outbound/chain"
)

// errorKind is the typed kind of one round's result against one endpoint, as the
// suspension tracker sees it. It lives here — not in chain — because endpoint
// health is THIS layer's concern: the tracker decides which outcomes count
// against a peer, and nothing outside this package consumes the verdict.
//
// The distinction that matters is WHO is to blame. A dial that never connected
// is strong evidence against the endpoint; a request that reached it and failed
// is weaker evidence; a caller that walked away, or a local limit that refused
// before anything was sent, is no evidence at all and must never evict a
// healthy peer — an overloaded process evicting its endpoints is how a local
// problem becomes a cluster-wide one.
type errorKind int8

const (
	// success means the round completed: the dial connected, or the request
	// was answered. It clears the endpoint's failure streak. A suspended
	// endpoint is NOT restored here: re-admission is timer-based (see [Tracker]),
	// so outcomes for a suspended endpoint are ignored as stale.
	success errorKind = iota

	// connectFailed means the dial itself failed: refused, unreachable, TLS
	// handshake broke. The strongest evidence this package can hold against an
	// endpoint, counted on its own low threshold.
	connectFailed

	// timeoutFailed means the TRANSPORT hit a read/write deadline: the peer did
	// not answer within the connection's budget. It is evidence against the
	// endpoint (counted on the request arm), but named apart from
	// [unknownFailed] because "the peer went silent" is a different operator
	// story from "the peer answered with an error".
	timeoutFailed

	// contextCanceled means the round was cut off before it settled because a
	// context ended — canceled or deadline exceeded. That context may be the
	// caller's, the chain's own call budget, or a peer's; the kind names the
	// signal, not the actor. It says nothing about the endpoint: the tracker
	// ignores it, and an outcome arriving while an endpoint is suspended never
	// ends or extends the cool-down.
	contextCanceled

	// resourceExhausted means one of OUR limits refused before anything was
	// sent: the pool was exhausted, an acquire timed out. Never counted against
	// an endpoint, for the same reason as [contextCanceled].
	resourceExhausted

	// unknownFailed means the round did not complete and no stronger attribution
	// applies — a protocol error, an adapter error, or an injected fault. It is
	// NOT proof the downstream answered: the tracker counts it against the
	// endpoint by policy (the higher request threshold), not because the cause is
	// known.
	unknownFailed
)

// String returns the outcome's name — the constant's own name, in snake_case —
// for logs and metric attributes.
func (v errorKind) String() string {
	switch v {
	case success:
		return "success"
	case connectFailed:
		return "connect_failed"
	case timeoutFailed:
		return "timeout_failed"
	case contextCanceled:
		return "context_canceled"
	case resourceExhausted:
		return "resource_exhausted"
	case unknownFailed:
		return "unknown_failed"
	}
	return "invalid"
}

// classifyError maps a round's error to a [errorKind]. The tracker calls it
// once per settled round.
//
// A [chain.ConnectError]-marked error is a connect failure — the strongest
// evidence. A local resource failure is no evidence at all, so it is named
// before any timeout check (an acquire that timed out must not read as the
// peer going silent). An I/O timeout ([chain.IsIOTimeout]) is the peer not
// answering within the connection's budget: evidence, weaker than a refused
// dial. A canceled or deadline-exceeded CONTEXT is a caller abort: the endpoint
// did not get to answer. Everything else is a downstream failure.
//
// A business answer (a 4xx-equivalent) never reaches here: the client adapter
// folds it to nil with [Tolerate] before the executor sees it, so the round is
// a plain success and the endpoint's streak is cleared the same way.
//
// Protection rejections (circuit open, rate limited) must not reach this
// function: they describe a round that never went out, so there is no peer to
// settle. A caller filters them before settling.
func classifyError(err error) errorKind {
	if err == nil {
		return success
	}
	if chain.IsConnectError(err) {
		return connectFailed
	}
	if chain.IsIOTimeout(err) {
		return timeoutFailed
	}
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return contextCanceled
	}
	if chain.IsResourceError(err) {
		return resourceExhausted
	}
	return unknownFailed
}
