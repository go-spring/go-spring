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

package loadbalance

import (
	kloadbalance "go-spring.org/cloud/governance/capability/loadbalance"
	"go-spring.org/cloud/governance/controlplane/source"
	"go-spring.org/spring/gs"
)

// This package registers the selection root's own beans: the default source and
// the center. The source is INJECTED, not configured here; with no source bean
// the default below arms the center with an empty document. Exactly one
// Source[kloadbalance.Settings] must exist; gs rejects the ambiguous
// injection.
func init() {
	gs.Provide(func() source.Source[kloadbalance.Settings] {
		return source.NewPushSource(kloadbalance.Settings{})
	}).Condition(gs.OnMissingBean[source.Source[kloadbalance.Settings]]()).Caller(1)

	gs.Provide(func(cp *gs.ContextProvider, mgr *kloadbalance.Manager,
		src source.Source[kloadbalance.Settings]) (*Center, error) {
		return NewCenter(cp.Context, mgr, src)
	}).
		Name("selectionCenter").
		Export(gs.As[gs.Rooter]()).
		Caller(1)
}
