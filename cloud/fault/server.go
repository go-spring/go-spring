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

package fault

import (
	"context"
	"sync/atomic"

	"go-spring.org/cloud/traffic"
)

// server.go is the INBOUND shell: [Server] is the state handle a server
// starter's middleware or interceptor captures at wiring time. It reads its OWN
// kernel pointer; the manager is not consulted per request. Its outbound
// counterpart is executor.go.

// Server is one inbound shell's state: the kernel it currently runs and the
// load-test propagator it was created with. Apart from the atomic kernel pointer
// — which [ServerManager.Refresh] pushes into — it is immutable.
//
// Create one with [ServerManager.NewServer] at wiring time and capture it: each
// call creates a state the manager will push into, so calling it per request
// would grow the registry. Close it when the middleware is torn down.
type Server struct {
	dir  *direction
	prop traffic.Propagator
	core atomic.Pointer[core]
}

// setCore is the direction's push entry point; see [Client.setCore].
func (s *Server) setCore(k *core) { s.core.Store(k) }

// Fire decides whether an inbound request should be set on fire, and applies the
// configured latency. It is the server-side fault seam — a gin middleware, a gRPC
// interceptor or a dubbo filter calls it before invoking its handler, mirroring
// how [Client.Fire] gates outbound calls. Both directions run the same injection
// sequence ([evaluate]) and the same guardrail model, but each shell reads its
// own kernel, so burning inbound requests leaves outbound calls untouched.
//
// It does NOT take the handler. The returned error is the "may I proceed?"
// answer: nil means run the handler, anything else means return it instead — an
// injected failure, or the context error when an injected latency was cancelled
// mid-sleep. Leaving the call to the adapter is what lets it choose the failure
// shape its protocol needs (a 503, a gRPC status, a failing RPC result) without
// having run the handler first, and it is why this package never has to guess
// what "failed" looks like on someone else's wire.
//
// t is the inbound call as the adapter can name it: the entrance this server
// presents ([Target.Service], "gin" / "grpc"), the interface of it being invoked
// ([Target.Action] — the HTTP route, the RPC method), and any request metadata
// the adapter chose to expose for matching ([Target.Metadata], often from
// headers). This package has no way to derive them — an inbound request's shape
// is the protocol's — so they are the adapter's to fill, and an Action left empty
// simply leaves that rule filter unused.
//
// The returned [Decision] is what the manager pushed: which rule governed,
// whether an error was injected, how much latency was applied. An adapter uses it
// to tell an injected failure from a cancelled delay, and — since an inbound
// request has nowhere else to record it — it is the only source for "this
// response was set on fire" (a header, a signal, a chaos controller's audit).
//
// The inbound side is fixed by which shell this is, so a caller cannot aim a fire
// at the outbound config. Fire does NOT retry — retry is a client concern; see
// [ClientManager.WrapClientExecutor].
func (s *Server) Fire(ctx context.Context, t Target) (Decision, error) {
	return evaluate(s.core.Load(), s.prop, t, ctx)
}

// Close unregisters the shell from its manager; see [Client.Close].
func (s *Server) Close() { s.dir.unregister(s) }
