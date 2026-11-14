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

// recorder.go is the WRITE-BACK half of a call: what the chain LEARNED while the
// call ran, as opposed to the declaration it was handed ([Operation], in
// operation.go). [Recorder] holds the ROUNDS — and nothing else: everything that
// happened during a round (its bound, its observability block, its fire, its
// peer) lives on that round, and the call-level view is derived from the array.
// It is written through a pointer as the call proceeds and is only valid to read
// once [Executor.Execute] returns.

package chain

import (
	"slices"
)

// Recorder is the write-back half of a call: what the chain LEARNED as the call
// ran, as opposed to the declaration ([Operation]), which is immutable and
// travels by value. Splitting them is what makes the declaration shareable — a
// layer that mutates its own copy of the Operation changes nothing anyone else
// sees — and gives every layer one explicit place to write.
//
// It exists because a call's outcome is not known to the caller who builds the
// Operation: only the chain sees what the downstream answered, whether protection
// rejected the call, which rule applied and whether a fault was injected.
// Funnelling all of it through one object — rather than a context value per
// concern — gives every layer one place to write and the caller one place to read.
//
// SCOPE: the rounds are the whole of this struct. A call is NATIVELY MULTI-ROUND
// — the first try plus however many retries ran — and everything that happened
// DURING a round (its bound, its peer, its fire, its produced signals, its
// measured outcome) lives on that round. The call-level view is DERIVED: the
// call's timeout and peer are the last round's, its fire the most recent round
// that fired, its produced signals the rounds' concatenated in order, and its
// own error is the one Execute RETURNS.
//
// The call's own error is the one Execute RETURNS and its status is the last
// round's status — readers derive both from the rounds.
//
// OWNERSHIP: one Recorder belongs to exactly ONE call. It is written by the chain
// on the calling goroutine (no lock: a nested call gets its own Recorder, not a
// shared one) and is only valid to READ after Execute returns. It must be
// non-nil: a caller that wants only the signals passes a record it simply never
// reads.
type Recorder struct {

	// Rounds holds the call's rounds, oldest first. len() is the count; empty means
	// the record was never handed to a chain (read before Execute, or never
	// passed to one) — NOT that the call was rejected, which records a round like
	// everything else. Created through [Recorder.NewRound], sealed
	// in place by the retry loop that created it; readers treat it as read-only.
	//
	// Rounds are held by POINTER because NewRound hands each one out as a live
	// handle: its address must stay valid for as long as a layer beneath retains
	// it, and a value slice would invalidate earlier handles when a later append
	// reallocates.
	Rounds []*Round
}

// NewRound appends a fresh round and returns the live handle the round's layers
// write into. It is called by the retry loop — the only layer that knows a round
// began — which fills in what it measures when the round ends, while the layers
// beneath write into it as the round runs (the peer they picked, the fire that
// hit, the signals the fn produced). Sealing is the retry loop's last write, not
// a separate record.
func (r *Recorder) NewRound() *Round {
	r.Rounds = append(r.Rounds, new(Round))
	return r.LastRound()
}

// LastRound returns the most recent round. Every call that reaches an executor
// has at least one — the first attempt, or the gate's refusal — so an empty
// recorder means the record was never handed to a chain, which is a misuse at
// the read site rather than a value to inspect.
//
// It is the write target for everything that happens "this round" and the read
// target for the call-level state: the call's own status, peer, timeout and
// error derive from it, so the derivation lives here rather than being spelled
// out again at every reader.
func (r *Recorder) LastRound() *Round {
	return r.Rounds[len(r.Rounds)-1]
}

// FailedAddrs lists, oldest first and deduplicated, the addresses of the
// rounds THIS call has already run against — the exclusion list the next
// round's pick is handed. It is read by the addressing layer at the start of a
// round: every round recorded by then failed (a success ends the call) and
// carries the peer it ran against (a refusal, or a pick that found nothing, ends
// the call rather than retrying). That precondition is why this is a plain
// projection of the rounds, with no filtering.
func (r *Recorder) FailedAddrs() []string {
	var out []string
	for _, rd := range r.Rounds {
		if slices.Contains(out, rd.Peer.Addr) {
			continue
		}
		out = append(out, rd.Peer.Addr)
	}
	return out
}
