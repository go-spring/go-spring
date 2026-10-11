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

// dial.go is this starter's connection-establishing seam. The mongo driver v2
// exposes no per-command execute hook — only SetDialer / SetMonitor — so the
// resilience executor is reachable only at DIAL time. Rather than push a seam
// with those semantics into the framework, it lives here, where its one user
// is: a client that has nowhere else to put protection.
//
// This is a degradation, not a design goal. The chain keys on the dial, so a
// breaker trips on connection failures (refused, timed out) rather than on
// per-request errors of an already-open connection, and pools make dials rare.
// A future client with a real per-command hook should not reach for this — it
// should take an executor through a request-level adapter instead.

package StarterMongoDB

import (
	"context"
	"net"
)

// dialFunc is the connection-establishing hook shape the mongo driver takes via
// options.Dialer — it matches net.Dialer.DialContext as well as the discovery
// dial closure the starter builds (pick a live endpoint from the current
// generation, then dial its Addr). [dialerWrapper.dial] is one.
type dialFunc func(ctx context.Context, network, addr string) (net.Conn, error)
