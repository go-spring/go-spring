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

package fault

// Document is the fault root's whole expected state: one armed rule list per
// traffic direction. The two directions are independent in effect — separate
// managers, separate counters — but they travel together because one operator
// arms a drill, and one push is easier to reason about than two.
//
// It IS the document the control-plane fault root binds (see
// cloud/governance/controlplane/fault): the root parses this type and fans it out
// to the two direction managers, so there is no separate binding twin.
//
// Bind via spring.fault.client.* and spring.fault.server.*, e.g.
//
//	spring.fault.client.rules[0].name=redis-errors
//	spring.fault.client.rules[0].service=redis:cache
//	spring.fault.client.rules[0].rate=0.5
//	spring.fault.client.rules[0].error=generic
//
// Every rule's Name is required within its direction (its expr tag rejects
// the empty spelling): it is the identity a push reconciles against and the
// name the call record carries.
type Document struct {
	// RunID labels ONE drill execution and is copied into BOTH directions before
	// they are pushed, so an operator arms an outbound and an inbound fire under
	// one id. Key: spring.fault.run-id.
	//
	// REQUIRED on every published document, armed or not, enforced by the field's
	// own binding (omitted fails the value tag, empty fails expr). Without an id a drill
	// cannot be reported, and two drills reusing a rule name would share one
	// counter identity, the second inheriting the first's spent blast radius. A
	// DISARM is still a drill event and names its run (`run-id=<same>` with no
	// rules), so "no id" is never a meaningful published state.
	//
	// A DIRECTLY constructed document (no binder) may therefore leave it empty,
	// falling back to the data plane's "no run id" semantics — the same escape
	// hatch a caller of [Manager.Refresh] has.
	//
	// A control plane mints it when a drill starts; it is what makes traces and
	// per-process counters from many replicas groupable into one execution. It is
	// document metadata, not a rule and not an effect.
	RunID string `value:"${spring.fault.run-id}" expr:"$ != ''"`

	// Client is the outbound rule list — fires this process's own calls.
	// Keys: spring.fault.client.*.
	Client Config `value:"${spring.fault.client:=}"`

	// Server is the inbound rule list — fires requests this process receives.
	// Independent of Client, including the guardrails.
	// Keys: spring.fault.server.*.
	Server Config `value:"${spring.fault.server:=}"`
}
