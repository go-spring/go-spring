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

// Package runtime is the process's governance composition root.
//
// It is deliberately NOT one of the two planes. The control plane
// ([go-spring.org/cloud/runtime/controlplane/governance]) owns one expected-state
// document: how it is fetched ([governance.Source]) and how it is distributed
// ([governance.Center]). The data plane
// ([go-spring.org/cloud/runtime/dataplane]) owns what happens on and around the
// request path: admission, resilience, fault injection, endpoint selection,
// observability and the in-process discovery directory. This package exists only
// to hold one instance of each and hand them out through a single injection
// point, so a starter never has to wire the family together itself.
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
// # Plane membership
//
// A package belongs to a plane by the execution system it serves, not by the
// role of the objects it defines. The data plane is the dependency closure of
// request execution, so the direction-neutral leaves it runs ([breaker],
// [ratelimit]) and the vocabulary its adapters share ([label], [traffic]) are
// part of it; "neutral" does not mean "outside the plane". A dependency leaves
// a plane only when a consumer outside the request path needs it too — a cache,
// a lock, a scheduler, a config store — and then it moves up to the nearest
// common ancestor. That is why cloud/observability/attrs and
// cloud/observability/conf sit outside runtime/.
//
// Two couplings follow, and they are deliberate rather than defects:
//
//   - The control-plane document embeds data-plane policy types
//     ([resilience.Policy], [guard.Policy], [loadbalance.Selection],
//     [fault.Config], [chain.Selector]), and [governance.Center] pushes them
//     into the concrete managers. An in-process control plane has no
//     independent deployment or document version, so type and configuration
//     unity is worth more than schema independence. The invariant to keep is
//     the one-way dependency below.
//   - The write side
//     ([go-spring.org/cloud/runtime/controlplane/registration.Instance]) and
//     the read side ([discovery.Endpoint]) keep separate vocabularies, synced
//     at the backend starters, which fold the instance's version, zone and
//     scheme into endpoint metadata under discovery's MetaKey* keys. Unifying
//     the two types is not planned.
//
// A [Runtime] holds both planes' objects but exposes them as authorities. It is
// the only exported composition surface: the control plane never exposes a
// data-plane authority, and a data-plane package never imports the control plane
// root.
package runtime
