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

// executor.go is the chain link that makes the endpoint decision part of a
// ROUND: it picks the peer the round runs against (re-picking on retry,
// excluding the peers this call already failed against), stages it on the
// recorder for the round's record, hands the peer to fn, and settles the
// round's outcome with the peer's owner exactly once. The Chain is the only
// thing that addresses, so this layer talks to it directly. Starters route with
// the peer they are handed and never settle — one round, one pick, one settle,
// one owner.
package loadbalance

import (
	"context"
	"errors"

	"go-spring.org/cloud/governance/dataplane/outbound/chain"
	balancer "go-spring.org/cloud/loadbalance"
)

type executor struct {
	c    *Chain
	next chain.Executor
}

// Wrap layers this package's addressing over next: the executor it returns
// sits INSIDE the retry loop (a composition root places it between the
// resilience executor and the fault executor) so every attempt picks its own
// peer. next nil is the terminal case and legal: the wrapped fn IS the call.
//
// A round refused before this layer (a gate, a budget) never reaches it; a pick
// that finds no endpoint fails the round with [chain.ErrNoEndpoints], which the
// retry loop's own round record classifies.
func Wrap(c *Chain, next chain.Executor) chain.Executor {
	return &executor{c: c, next: next}
}

// Execute runs one round of addressing: pick (avoiding the addresses this
// call's earlier rounds failed against), stage the peer, run the rest of the
// chain, settle the peer with the round's outcome. The settle uses the error
// the round returned, whatever classified it — an injected fault settles as a
// fault, a dial as a dial ([classifyError] sorts them out), and a protection
// rejection never reaches here because it never rounds.
func (e *executor) Execute(ctx context.Context, op chain.Operation, rec *chain.Recorder, fn chain.RoundFunc) error {
	// The routing input a round can state: the zone and affinity key from the
	// caller's per-call options (see [chain.WithOptions]) — the caller's handle
	// for targeting a downstream zone/cluster and for keying affinity. Affinity
	// is the caller's decision alone: a starter never derives a key on the
	// caller's behalf, so an unkeyed call picks unkeyed (strategies degrade by
	// design — consistent hash falls back to round-robin). Everything else a
	// balancer needs — the service, the selection — is fixed by WHICH chain this
	// is.
	var info balancer.PickInfo
	if o, ok := chain.OptionsFrom(ctx); ok {
		info.Zone = o.Zone
		info.HashKey = o.HashKey
	}
	peer, err := e.c.Pick(ctx, info, rec.FailedAddrs())
	if err != nil {
		// The balancer speaks its own "nothing to pick" word; the chain records
		// failures in ITS vocabulary, so the boundary translates rather than
		// leaking a backend error the retry loop cannot classify.
		if errors.Is(err, balancer.ErrNoEndpoints) {
			return chain.ErrNoEndpoints
		}
		return err
	}
	// Stage the pick on the round the loop opened: the peer is both the live
	// handoff [chain.Next]'s terminal passes to fn and the flat fact readers
	// query ([chain.Peer.Addr]).
	rec.LastRound().Peer = peer
	defer func() { peer.Complete(ctx, err) }()
	// Assign before returning: the deferred settle reads this variable, not the
	// return slot.
	err = chain.Next(e.next, ctx, op, rec, fn)
	return err
}

// Close forwards to the next layer. Addressing holds no state of its own: the
// picker's state belongs to the picker's owner.
func (e *executor) Close() error {
	if e.next == nil {
		return nil
	}
	return e.next.Close()
}
