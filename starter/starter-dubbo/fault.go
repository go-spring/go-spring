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
	"go-spring.org/cloud/fault"
	"go-spring.org/cloud/governance"
)

const faultFilterKey = "fault"

// injector is the governance starter's fault injector, installed once at wiring
// time by newInjectorHook. It is a package handle rather than an injected field
// because dubbo — not this starter — constructs filter instances through its own
// registry, so there is no bean whose constructor could receive the injector.
// No ordering constraint applies: the handle holds the injector BEAN, and the
// injector swaps its config in place ([fault.Injector.SetConfig]), so whatever
// the center pushes later is visible to the filter without re-installing.
//
// nil (the governance center not imported) means fault injection is off: the filter
// passes the call through untouched.
var injector atomic.Pointer[fault.Injector]

// injectorHook is the marker bean whose construction installs the injector.
type injectorHook struct{}

// newInjectorHook installs the center's fault authority as the injector the
// fault filter reads. center is nil when none is linked, which leaves fault
// injection off and the filter transparent; installing unconditionally (rather
// than skipping a nil) keeps the handle from surviving a container that did have
// one — a test-process concern, but the same "last wiring wins" rule in both
// cases.
func newInjectorHook(center *governance.Center) (*injectorHook, error) {
	injector.Store(center.Fault())
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
