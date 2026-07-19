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

package StarterTransactionTCC

// Config binds ${spring.transaction.tcc}. It configures the bundled in-process
// TCC coordinator contributed by this starter. The prefix sits under the shared
// spring.transaction capability namespace (alongside spring.transaction.saga),
// so an application may enable Saga and TCC side by side without a collision.
type Config struct {
	// Enabled turns the starter's beans on. It defaults to true, so a blank import
	// is enough; set it to false to import the module without contributing beans.
	Enabled bool `value:"${enabled:=true}"`

	// Tracing wires the otel [Observer] so each participant phase emits a child
	// span (try, confirm, cancel) on the globals installed by starter-otel. It
	// defaults to true and costs almost nothing without starter-otel, since the
	// global tracer is then a no-op.
	Tracing bool `value:"${tracing:=true}"`

	// RecoverOnStart makes the starter scan the durable Store at startup and drive
	// any transaction left in flight by a crash to its decided outcome (confirm if
	// a commit decision was recorded, otherwise cancel). It defaults to true. With
	// the in-memory default Store the scan is always empty (a restart loses the
	// log), so this only does real work once a durable Store starter is imported.
	RecoverOnStart bool `value:"${recover-on-start:=true}"`
}
