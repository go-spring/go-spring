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

// Package governance is the process's service-governance runtime: the
// DATA-PLANE composition root. Governance names the whole layer — traffic
// policy, fault injection, endpoint selection, discovery and the
// observability of the governed call — while its control plane is a set of
// peer ROOTS (policy and fault), never one document.
//
// It is deliberately NOT one of the two planes. The control plane is a set of
// PEER config roots, each owning its own expected-state document: the three
// outbound protection capabilities — limits
// ([go-spring.org/cloud/governance/controlplane/timeoutretry]), breaker and rate limit —
// endpoint selection
// ([go-spring.org/cloud/governance/controlplane/loadbalance]), inbound admission
// ([go-spring.org/cloud/governance/controlplane/admission]) and fault
// ([go-spring.org/cloud/governance/controlplane/fault], experiments). Each has its
// own document, source and last-good. They share the transport
// ([go-spring.org/cloud/governance/controlplane/source]) but not the document, the
// writer or the validation. The data plane
// ([go-spring.org/cloud/governance/dataplane]) owns what happens on and around the
// request path: admission, resilience, fault injection, endpoint selection,
// observability and the in-process discovery directory. This package exists only
// to hold one instance of each DATA-PLANE authority and hand them out through a
// single injection point, so a starter never has to wire the family together
// itself. It holds no control-plane center: a center arms the managers this
// object hands out, without the runtime knowing it.
//
// Control-plane topology is peers; data-plane topology is layers. The two trees
// meet only here.
//
// The division that matters here is the one the planes do NOT absorb:
//
//   - Fetching a document from a remote center is control-plane work; the
//     transport adapters (etcd, nacos, …) live in the starter modules.
//   - Registering this process and resolving its peers is data-plane work: the
//     directory and the publication lifecycle are process-local state. The
//     remote registry clients are starter adapters too.
//   - Observability is data plane because it implements the governed call: it
//     wraps [chain.Executor] for outbound calls and drives the server's
//     Admit/Done pair. It is the OUTERMOST data-plane layer, which is why it may
//     depend on [guard] and [breaker]; nothing depends on it on the request
//     path.
//
// # Membership
//
// A package belongs where the execution system it serves puts it, not where the
// role of its objects would. A package that knows a call direction belongs to
// that direction's tree under dataplane/. A package that knows neither a
// direction nor a document source — the breaker, ratelimit, fault, traffic,
// label, discovery, loadbalance and matcher mechanisms and vocabularies — is a
// capability: capability/ is a tier of its own, consumed by both planes but
// owned by neither. The two planes' topology therefore meets a third,
// plane-neutral one, and only this package holds all three.
//
// A package leaves governance/ only when a consumer outside the governance
// runtime needs it too — a cache, a lock, a scheduler, a config store — and
// then it moves up to the nearest common ancestor. That is why
// cloud/observability/attrs and cloud/observability/conf sit outside
// governance/.
//
// Two couplings follow, and they are deliberate rather than defects:
//
//   - Each control-plane document embeds ITS data-plane types —
//     [resilience.Limits], [resilience.BreakerPolicy],
//     [resilience.RateLimitPolicy], [admission.Policy] and
//     [loadbalance.Selection]; [fault.Config] for fault — and each root's
//     center pushes them into the concrete managers. An in-process
//     control plane has no independent deployment or document version, so type
//     and configuration unity is worth more than schema independence. The
//     invariant to keep is the one-way dependency below.
//   - The write side
//     ([go-spring.org/cloud/governance/controlplane/registration.Instance]) and
//     the read side ([discovery.Endpoint]) keep separate vocabularies, synced
//     at the backend starters, which fold the instance's version, zone and
//     scheme into endpoint metadata under discovery's MetaKey* keys. Unifying
//     the two types is not planned.
//
// A [Runtime] holds only the data plane's objects, exposed as authorities. It is
// the only exported composition surface a starter injects: a data-plane package
// never imports a control-plane root, and a control-plane root reaches the
// managers it drives directly, not through this object.
package governance
