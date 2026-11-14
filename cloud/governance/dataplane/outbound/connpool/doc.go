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

// Package connpool is the outbound CONNECTION-HOLDING layer: per-endpoint
// sub-pools with bounded concurrency, idle parking and lazy reaping, for
// clients that have no vendor pool of their own (redigo). A client that HAS
// one (go-redis, database/sql, mongo) never builds a Pool — its dial hook
// reads the address the chain staged (see [WithDialAddr]) and the vendor pool
// keeps managing connections.
//
// # What this layer does NOT own
//
// Addressing is the chain's: the addressing stage picks the endpoint —
// re-picking on retry, excluding the peers the call already failed against —
// and hands the address to [Pool.Acquire]. Health and suspension live in the
// balancer's tracker; a dial failure is MARKED here ([chain.ConnectError]), and
// the chain's round settlement is the one place it is read. This layer's only
// vocabulary is acquire, dial, park, close.
//
// A local exhaustion ([ErrPoolExhausted]) is never an endpoint's fault: the
// chain settles it as no evidence against the peer.
//
// The package is a leaf in the sense that matters: it imports the chain call
// shape and OTel, and no governance engine or balancer.
package connpool
