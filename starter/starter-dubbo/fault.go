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

package StarterDubbo

import (
	"context"
	"sync/atomic"

	"dubbo.apache.org/dubbo-go/v3/filter"
	"dubbo.apache.org/dubbo-go/v3/protocol/base"
	"dubbo.apache.org/dubbo-go/v3/protocol/result"
	"go-spring.org/cloud/governance"
	"go-spring.org/cloud/governance/capability/fault"
	serverfault "go-spring.org/cloud/governance/dataplane/inbound/fault"
)

const faultFilterKey = "fault"

// faultServer is the inbound shell the filter fires through, created once at
// wiring time by newInjectorHook. It is a package handle rather than an injected
// field because dubbo — not this starter — constructs filter instances through
// its own registry, so there is no bean whose constructor could receive it.
// No ordering constraint applies: the shell holds its own kernel pointer, and
// the authority swaps that kernel in place ([serverfault.Manager.Refresh]), so
// whatever the runtime pushes later is visible to the filter without
// re-installing.
//
// dubbo is the ONE inbound adapter whose fault stage is standalone rather than
// folded into the admission: it keeps its own URL-param governance model and does
// not drive a admission.Guard at all. There is therefore no second injection path
// to double-fire — this shell is the only inbound fault seam dubbo has.
//
// nil (the runtime not imported) means fault injection is off: the filter
// passes the call through untouched.
var faultServer atomic.Pointer[serverfault.Server]

// injectorHook is the marker bean whose construction installs the shell.
type injectorHook struct{}

// newInjectorHook installs the center's inbound fault authority's shell, which
// the fault filter reads. center is nil when none is linked, which leaves fault
// injection off and the filter transparent; installing unconditionally (rather
// than skipping a nil) keeps the handle from surviving a container that did have
// one — a test-process concern, but the same "last wiring wins" rule in both
// cases.
func newInjectorHook(center *governance.Runtime) (*injectorHook, error) {
	faultServer.Store(center.FaultServer().NewServer())
	return &injectorHook{}, nil
}

type dubboFaultFilter struct{}

func newFaultFilter() filter.Filter { return &dubboFaultFilter{} }

// Invoke gates the service call with [serverfault.Server.Fire] when an injector is
// installed. The injector is read per call so fault can be hot-toggled at
// runtime without a restart (the handle itself never changes after wiring); nil
// means no fault is configured and the call passes through untouched.
//
// The call is named for the rules as the entrance ("dubbo") plus the method
// being invoked, so a rule can target one method instead of the whole provider.
func (f *dubboFaultFilter) Invoke(ctx context.Context, invoker base.Invoker, inv base.Invocation) result.Result {
	srv := faultServer.Load()
	if srv == nil {
		return invoker.Invoke(ctx, inv)
	}
	if _, err := srv.Fire(ctx, faultTarget(inv)); err != nil {
		// The call never reached the invoker: surface the injected failure (or
		// the cancelled-latency context error) itself, so its kind survives into
		// the result rather than collapsing to a generic one.
		return &result.RPCResult{Err: err}
	}
	return invoker.Invoke(ctx, inv)
}

// faultTarget names the inbound call for the fault rules: the entrance
// ("dubbo") plus the method the invocation names — the finest unit a rule can
// target. The method is the ACTION, not the service, so one rule can cover the
// whole provider and another a single method of it.
func faultTarget(inv base.Invocation) fault.Target {
	return fault.Target{
		Service: "dubbo",
		Action:  inv.MethodName(),
	}
}

// OnResponse is a pass-through.
func (f *dubboFaultFilter) OnResponse(_ context.Context, res result.Result, _ base.Invoker, _ base.Invocation) result.Result {
	return res
}
