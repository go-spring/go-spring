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

// Package observability is the OUTERMOST data-plane layer: it instruments the
// governed call. Outbound, [WrapClientExecutor] wraps a [chain.Executor] so
// every attempt, rejection and fault is traced, measured and logged; inbound,
// [ServerObserver] opens the request span and settles the observation around
// the guard's Admit/Done pair. It may therefore depend on chain, breaker and
// guard — nothing on the request path depends on it.
//
// The neutral halves live OUTSIDE the planes, at
// `cloud/observability/attrs` (the per-request attribute carrier
// and the shared duration buckets) and
// `cloud/observability/conf` (the property-refresh funnel).
// They import no governance code, so a cache, a lock, a scheduler, a config
// starter or the control-plane registration package can use them without
// pulling chain/guard/breaker into its dependency graph. Leaving them under
// dataplane/ would have reclassified neutral vocabulary as a data-plane layer.
package observability
