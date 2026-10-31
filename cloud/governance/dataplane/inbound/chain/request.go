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

// Package chain is the inbound data plane's seam: the request vocabulary and the
// fault-injection interface the observer, admission and fault stages share. It
// depends on no stage, so each stage depends on this package instead of on its
// callers — the inbound mirror of outbound/chain.
//
// The rule is the same as on the outbound side: a stage may import chain, chain
// may not import a stage. The decision type the fault seam names comes from
// capability/fault, the shared base layer both planes build on.
package chain

// Request is one inbound request as the seams can name it. It is a value, not a
// handle: a stage reads it and keeps nothing from it.
type Request struct {
	// Action is the interface being invoked, in the protocol's own format:
	// "GET /users/{id}" for an HTTP route (the TEMPLATE, not the concrete URL),
	// "/pkg.Svc/Method" for an RPC. Empty means the adapter could not name it; a
	// rule that narrows on Action then simply does not match.
	Action string

	// Metadata is the request's targeting set — typically request headers the
	// deployment cares about ("tenant", "region", "canary"). It is matched by a
	// rule and never emitted. Nil means no targeting metadata.
	Metadata map[string]string
}
