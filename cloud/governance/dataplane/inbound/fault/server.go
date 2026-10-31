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

	kfault "go-spring.org/cloud/governance/capability/fault"
)

// server.go is the INBOUND seam: [Server] is the entry point a server starter's
// middleware or interceptor captures at wiring time. It loads the authority's
// published kernel pointer; the manager's lock is not taken per request. Its
// outbound counterpart is
// [go-spring.org/cloud/governance/dataplane/outbound/fault.Client].

// Server is the inbound entry point an adapter fires through: it holds the
// inbound authority and exposes only Fire, so the caller cannot aim a fire at
// the outbound config or reach the authority's Refresh/Config.
//
// Create one with [Manager.NewServer] at wiring time and capture it.
type Server struct {
	mgr *kfault.Manager
}

// Fire decides whether an inbound request should be set on fire, and applies the
// configured latency. It is the server-side fault seam — a gin middleware, a gRPC
// interceptor or a dubbo filter calls it before invoking its handler, mirroring
// how [go-spring.org/cloud/governance/dataplane/outbound/fault.Client.Fire] gates
// outbound calls. Both directions run the same injection sequence and the same
// guardrail model, but each direction reads its own kernel, so burning inbound
// requests leaves outbound calls untouched.
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
// presents ([kfault.Target.Service], "gin" / "grpc"), the interface of it being
// invoked ([kfault.Target.Action] — the HTTP route, the RPC method), and any
// request metadata the adapter chose to expose for matching
// ([kfault.Target.Metadata], often from headers). This package has no way to
// derive them — an inbound request's shape is the protocol's — so they are the
// adapter's to fill, and an Action left empty simply leaves that rule filter
// unused.
//
// The traffic class is one of those metadata entries: a middleware building an
// [go-spring.org/cloud/governance/dataplane/inbound/observe.ServerOperation] with
// its base constructor already records it, and passes the same map here. A
// Target without the "traffic" entry does not match a rule naming match.traffic.
//
// The returned [kfault.Result] is what the manager pushed: which rule governed,
// whether an error was injected, how much latency was applied. An adapter uses it
// to tell an injected failure from a cancelled delay, and — since an inbound
// request has nowhere else to record it — it is the only source for "this
// response was set on fire" (a header, a signal, a chaos controller's audit).
//
// The inbound side is fixed by which authority this is, so a caller cannot aim
// a fire at the outbound config. Fire does NOT retry — retry is a client concern.
func (s *Server) Fire(ctx context.Context, t kfault.Target) (kfault.Result, error) {
	return s.mgr.Fire(ctx, t)
}
