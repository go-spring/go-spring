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

package admission

import (
	"go-spring.org/cloud/governance/controlplane/source"
	"go-spring.org/cloud/governance/dataplane/inbound/admission"
	"go-spring.org/spring/gs"
)

// This package registers the admission root's own beans: the default source and
// the center. The source is INJECTED, not configured here; with no source bean
// the default below arms the center with an empty document (every Guard admits
// with no protection). Exactly one Source[admission.Settings] must exist.
func init() {
	gs.Provide(func() source.Source[admission.Settings] {
		return source.NewPushSource(admission.Settings{})
	}).Condition(gs.OnMissingBean[source.Source[admission.Settings]]()).Caller(1)

	gs.Provide(func(cp *gs.ContextProvider, mgr *admission.Manager,
		src source.Source[admission.Settings]) (*Center, error) {
		return NewCenter(cp.Context, mgr, src)
	}).
		Name("admissionCenter").
		Export(gs.As[gs.Rooter]()).
		Caller(1)
}
