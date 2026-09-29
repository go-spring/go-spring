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

	"dubbo.apache.org/dubbo-go/v3/common/extension"
	"dubbo.apache.org/dubbo-go/v3/filter"
	"dubbo.apache.org/dubbo-go/v3/protocol/base"
	"dubbo.apache.org/dubbo-go/v3/protocol/result"
	"go-spring.org/cloud/governance/fault"
	"go-spring.org/spring/gs"
)

func init() {
	// Register the inbound fault-injection filter under "fault". Add "fault" to
	// a provider's filter chain to activate it. dubbo's filter registry hands out
	// filters via a no-arg constructor, so the injector cannot arrive as a
	// constructor parameter: a bean installs it into the package handle below
	// instead.
	extension.SetFilter(faultFilterKey, newFaultFilter)

	// The install-and-hold bean. Exported as a gs.Rooter so gs instantiates it
	// even though nothing injects it — without a collected-type export an
	// unreachable bean is never created and the filter would stay a pass-through.
	//
	// The injector is a NULLABLE injection: it exists whenever starter-governance
	// is in the container, which is the normal case, and is absent from a
	// container without it. Without the "?" gs would treat an absent bean as a
	// wiring error and the app would not boot — turning "governance is off" into
	// "governance must be imported", which is not the contract. A nil injector
	// leaves fault injection off and the filter transparent.
	gs.Provide(newInjectorHook,
		gs.IndexArg(0, gs.TagArg("?")),
	).Export(gs.As[gs.Rooter]()).Caller(1)
}

const faultFilterKey = "fault"

// injector is the governance starter's fault injector, installed once at wiring
// time by newInjectorHook. It is a package handle rather than an injected field
// because dubbo — not this starter — constructs filter instances through its own
// registry, so there is no bean whose constructor could receive the injector.
// No ordering constraint applies: the handle holds the injector BEAN, and the
// injector swaps its config in place ([fault.Injector.SetConfig]), so whatever
// the center pushes later is visible to the filter without re-installing.
//
// nil (starter-governance not imported) means fault injection is off: the filter
// passes the call through untouched.
var injector atomic.Pointer[fault.Injector]

// injectorHook is the marker bean whose construction installs the injector.
type injectorHook struct{}

// newInjectorHook installs inj as the injector the fault filter reads. inj is
// nil when starter-governance is not imported, which leaves fault injection off
// and the filter transparent; installing unconditionally (rather than skipping a
// nil) keeps the handle from surviving a container that did have one — a
// test-process concern, but the same "last wiring wins" rule in both cases.
func newInjectorHook(inj *fault.Injector) (*injectorHook, error) {
	injector.Store(inj)
	return &injectorHook{}, nil
}

type dubboFaultFilter struct{}

func newFaultFilter() filter.Filter { return &dubboFaultFilter{} }

// Invoke gates the service call with [fault.ApplyServer] when an injector is
// installed. The injector is read per call so fault can be hot-toggled at
// runtime without a restart (the handle itself never changes after wiring); nil
// means no fault is configured and the call passes through untouched.
func (f *dubboFaultFilter) Invoke(ctx context.Context, invoker base.Invoker, inv base.Invocation) result.Result {
	inj := injector.Load()
	if inj == nil {
		return invoker.Invoke(ctx, inv)
	}
	var res result.Result
	_ = fault.ApplyServer(ctx, inj, "dubbo", func() error {
		res = invoker.Invoke(ctx, inv)
		return nil
	})
	if res != nil {
		return res
	}
	// Apply injected an error before invoker ran; surface a failing result.
	return &result.RPCResult{Err: fault.ErrInjected}
}

// OnResponse is a pass-through.
func (f *dubboFaultFilter) OnResponse(_ context.Context, res result.Result, _ base.Invoker, _ base.Invocation) result.Result {
	return res
}
