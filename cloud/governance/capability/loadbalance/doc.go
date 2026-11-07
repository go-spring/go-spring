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

// Package loadbalance is the client-side load-balancing layer that sits on top
// of [go-spring.org/cloud/discovery].
//
// Discovery answers "which instances exist right now?"; this package answers
// the next question — "given that live set, which one do I send this request
// to?". It is deliberately split into two concerns:
//
//   - [Balancer] is the pluggable selection strategy (round-robin, least-conn,
//     consistent-hash, weighted, random, p2c). It is pure: given a candidate
//     endpoint set and a [PickInfo] it returns one endpoint, plus the
//     Complete method that settles the request it issued. [ZoneAwareBalancer] is
//     a filter that wraps any strategy when the selection asks for locality.
//   - [Pool] binds a live discovery source (a [discovery.Resolver] bound to one
//     service name via [discovery.NewResolver]) and a
//     [Tracker] (outlier suspension) to a Balancer, so the candidate set stays
//     fresh as instances come and go and unhealthy instances are suspended.
//
// A strategy names itself in a selection rule and is resolved by a [Factory]
// from the [Provider] beans wiring contributes: this package contributes one
// via [NewBuiltinProvider], and a deployment adds its own by contributing
// another Provider bean. The core fixes only the
// [Balancer]/[Provider]/[Selection.Params] contract, so a new strategy — with
// parameters of its own — is added entirely outside this package.
//
// The package has zero third-party dependencies; RPC-framework adapters (gRPC
// balancer.Builder, kitex loadbalance.Loadbalancer, ...) live in their starters
// and translate a Balancer into the framework's own picker interface.
package loadbalance
