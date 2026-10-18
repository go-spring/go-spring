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

// Package registration is the CONTROL-PLANE write side of service discovery:
// it publishes this process into the configured naming centers and maintains
// that publication across the gs.Server lifecycle.
//
// It owns desired endpoint state, not the request path: a business request
// never enters this package. The read side — the in-process directory that
// load balancing consults on every call — is
// [go-spring.org/cloud/runtime/dataplane/kernel/discovery]. Each backend starter
// implements [Registry] for one center; [Server] drives all of them in
// lockstep, because a process published to some centers but not others would
// split consumers' view.
//
// [Instance] mirrors the read side's endpoint rather than sharing its type.
// The two planes keep separate vocabularies, synced at the backend starters;
// this is a deliberate boundary, not an unfinished unification.
//
// See README.md for the placement rationale.
package registration
