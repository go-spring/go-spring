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

package observe

import (
	"sync"

	"go-spring.org/cloud/governance/dataplane/outbound/chain"
)

// successQuiet reports whether a successful call's access line is logged at
// Debug (true) or Info (false).
//
// An UNDECLARED call is this layer's historical quiet case: it has always logged
// success at Debug, and a client that declares nothing (redigo's skipped PING,
// say) declares nothing precisely to keep the noise down — so it stays quiet.
// A declared call says so itself, via [chain.Observe.Log.QuietSuccess]: a producer
// whose calls carry high-frequency per-call detail sets it, one whose success is
// worth a line does not.
func successQuiet(hasOp bool, op chain.Operation) bool {
	if !hasOp {
		return true
	}
	return op.Observe.Log.QuietSuccess
}

// ResetInstruments drops the memoized outbound instrument sets so the next use
// resolves fresh ones. It exists for tests that install their own
// MeterProvider: the sets are process-wide and resolved once, so a test running
// after one that already resolved them would otherwise keep reporting into the
// earlier provider.
//
// It covers both the always-on client set and the per-prefix operation sets,
// since a MeterProvider swap invalidates all of them together.
func ResetInstruments() {
	clientInstruments = sync.OnceValue(buildClientInstruments)
	resetOperationInstruments()
}

// resetOperationInstruments drops the memoized per-prefix sets, for the same
// reason [ResetInstruments] does.
func resetOperationInstruments() {
	opInstruments.mu.Lock()
	defer opInstruments.mu.Unlock()
	opInstruments.m = map[string]*opInstrumentSet{}
}
