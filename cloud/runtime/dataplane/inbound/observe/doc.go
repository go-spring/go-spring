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

// Package observe is the INBOUND data plane's outermost layer: [ServerObserver]
// opens the request span and settles the observation around the admission
// authority's Admit/Done pair.
//
// It is a subpackage of inbound/, beside the admission authority it observes:
// the direction owns the whole stack, while the authority itself stays free of
// instrumentation. Nothing in admission imports this package; the composition
// root (cloud/runtime) is what stacks it.
//
// The neutral halves — the per-request attribute carrier, the meter/tracer
// scopes and the log tag — live outside the planes at
// `cloud/observability/attrs` and `cloud/observability/instrument`.
package observe
