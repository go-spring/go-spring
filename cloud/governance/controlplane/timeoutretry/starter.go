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

package timeoutretry

import (
	"go-spring.org/cloud/governance/controlplane/source"
	"go-spring.org/cloud/governance/dataplane/outbound/resilience"
	"go-spring.org/spring/gs"
)

// This package registers the limits root's own beans: the default source and
// the center. The source is INJECTED, not configured here: the framework ships
// the Source implementations, but which transport a deployment uses is owned by
// whoever owns the deployment's configuration. With no source bean the default
// below arms the center with an empty document — a process with no limits
// transport runs ungoverned instead of failing. Exactly one
// Source[resilience.LimitsSettings] must exist; gs rejects the ambiguous
// injection.
func init() {
	// Default source, stepping aside when a company-level starter provides one.
	gs.Provide(func() source.Source[resilience.LimitsSettings] {
		return source.NewPushSource(resilience.LimitsSettings{})
	}).Condition(gs.OnMissingBean[source.Source[resilience.LimitsSettings]]()).Caller(1)

	// The center is constructor-injected with the outbound resilience authority
	// and this root's source; [NewCenter] arms it on the spot. It is exported as
	// a root so gs constructs it even when nothing injects it.
	gs.Provide(func(cp *gs.ContextProvider, mgr *resilience.Manager,
		src source.Source[resilience.LimitsSettings]) (*Center, error) {
		return NewCenter(cp.Context, mgr, src)
	}).
		Name("limitsCenter").
		Export(gs.As[gs.Rooter]()).
		Caller(1)
}
