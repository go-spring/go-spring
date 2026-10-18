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

// Package observe is the OUTBOUND data plane's outermost layer: it instruments
// the governed call. [WrapClientExecutor] wraps a [chain.Executor] so every
// attempt, rejection and fault is traced, measured and logged.
//
// It is a subpackage of outbound/, beside the resilience authority it wraps:
// the direction owns the whole stack, while the authority itself stays free of
// instrumentation. Nothing in resilience or chain imports this package; the
// composition root (cloud/runtime) is what stacks it.
//
// The neutral halves — the per-request attribute carrier, the meter/tracer
// scopes and the log tag — live outside the planes at
// `cloud/observability/attrs` and `cloud/observability/instrument`.
package observe
