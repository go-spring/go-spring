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

package StarterTransactionATGorm

// Config binds ${spring.transaction.at}. It configures the AT
// distributed-transaction beans this starter contributes. The prefix sits under
// the shared spring.transaction capability namespace (alongside
// spring.transaction.saga and spring.transaction.tcc), so an application may
// enable several patterns side by side without a collision.
type Config struct {
	// Enabled turns the starter's beans on. It defaults to true, so a blank import
	// is enough; set it to false to import the module without contributing beans.
	Enabled bool `value:"${enabled:=true}"`

	// Tracing wires the otel [at.Observer] so each branch's second-phase operation
	// emits a child span (commit, rollback) on the globals installed by
	// starter-otel. It defaults to true and costs almost nothing without
	// starter-otel, since the global tracer is then a no-op.
	Tracing bool `value:"${tracing:=true}"`
}
