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

// Package traffic is the load-test identification contract for go-spring. It
// answers one question — "is the request in flight a load-test request?" — as a
// single in-process hook, [IsLoadTest], that business code reads.
//
// Why a hook. In any one process there is exactly one way to decide load-test
// traffic, so a single process-wide install point is coherent. go-spring's own
// convention (a context marker propagated across HTTP headers and gRPC
// metadata) is implemented in the sibling canonical package, which installs it
// into [IsLoadTest] on import. A company whose load generator or gateway stamps
// synthetic load with its own marker does not import canonical; it installs its
// own [IsLoadTest] instead.
//
// Business code never constructs the marker — it only reads [IsLoadTest]. How
// the flag got there (an HTTP header, a metadata key, a message property) is out
// of this package's concern.
package traffic

import "context"

// IsLoadTest reports whether the request in ctx is load-test traffic. It is
// installed once at startup: the canonical sibling package installs go-spring's
// default (a context marker), or a company installs its own to recognise its own
// convention. Before anything installs, nothing is load-test traffic — the
// default returns false.
var IsLoadTest = func(context.Context) bool { return false }
