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

import (
	kfault "go-spring.org/cloud/governance/capability/fault"
	"go-spring.org/cloud/governance/controlplane/source"
	serverfault "go-spring.org/cloud/governance/dataplane/inbound/fault"
	clientfault "go-spring.org/cloud/governance/dataplane/outbound/fault"
	"go-spring.org/spring/gs"
)

// This package registers the fault root's own beans: the default source and the
// center. The authority managers the center drives are registered by their own
// data-plane packages.
//
// The source is INJECTED, not configured here. The framework ships the Source
// implementations (file, http, push), but which transport a deployment uses —
// a local file, a config center, a control-plane stream — is owned by whoever
// owns the deployment's configuration, and is wired by that company-level source
// starter. A root does not ship its own transport modules: doing so would make
// every root repeat the same transport wiring, and would make two configured
// source modules collide.
//
// With no source bean the default below keeps the root armed with an empty
// document — a process with no fault source runs ungoverned instead of failing.
// Exactly one Source[kfault.Document] must exist; gs rejects the ambiguous injection
// rather than picking one.
func init() {
	// Default source: the center needs a source to arm from even when the process
	// configures no transport. This bean steps aside when a company-level starter
	// provides its own Source[kfault.Document] (the condition runs after modules).
	gs.Provide(func() source.Source[kfault.Document] {
		return source.NewPushSource(kfault.Document{})
	}).Condition(gs.OnMissingBean[source.Source[kfault.Document]]()).Caller(1)

	// The CENTER is constructor-injected: gs hands it the two authority beans and
	// this root's source, and [NewCenter] arms it on the spot. A bad document
	// never reaches it — the source adapter rejects what conf.Bind's expr
	// tags refused, failing startup there. It is exported as a root so gs constructs
	// it even when nothing injects it: no other bean holds it — the runtime only
	// DependsOn it, deliberately.
	gs.Provide(func(cp *gs.ContextProvider, client *clientfault.Manager, server *serverfault.Manager,
		src source.Source[kfault.Document]) (*Center, error) {
		return NewCenter(cp.Context, client, server, src)
	}).
		Name("faultCenter").
		Export(gs.As[gs.Rooter]()).
		Caller(1)
}
