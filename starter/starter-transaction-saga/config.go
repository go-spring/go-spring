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

package StarterTransactionSaga

// Config binds ${spring.transaction.saga}. It configures the bundled in-process
// Saga coordinator contributed by this starter. The capability-level prefix
// (spring.transaction, not spring.transaction.saga-memory) is intentional: a
// future TCC/AT variant shares it.
type Config struct {
	// Enabled turns the starter's beans on. It defaults to true, so a blank import
	// is enough; set it to false to import the module without contributing beans.
	Enabled bool `value:"${enabled:=true}"`

	// Tracing wires the otel [Observer] so each Saga step emits a child span
	// (one for the action, one for each compensation) on the globals installed by
	// starter-otel. It defaults to true and costs almost nothing without
	// starter-otel, since the global tracer is then a no-op.
	Tracing bool `value:"${tracing:=true}"`

	// RecoverOnStart makes the starter scan the durable Store at startup and
	// compensate any saga left in flight by a crash (backward recovery). It
	// defaults to true. With the in-memory default Store the scan is always empty
	// (a restart loses the log), so this only does real work once a durable Store
	// starter — e.g. starter-transaction-saga-gorm — is imported.
	RecoverOnStart bool `value:"${recover-on-start:=true}"`
}
