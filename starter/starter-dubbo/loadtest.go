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
	"go-spring.org/cloud/governance/traffic"
	"go-spring.org/cloud/propagate"
	"go-spring.org/spring/gs"
)

func init() {
	// Register the load-test identification filter under the dubbo filter name
	// "loadtest". Activate it by adding "loadtest" to a provider's filter chain
	// (ideally first), so the marker is on the context before later filters and
	// the service impl run — letting downstream code branch on the load-test
	// convention.
	extension.SetFilter(loadTestFilterKey, newLoadTestFilter)

	// The install-and-hold bean. dubbo's filter registry hands out filters via a
	// no-arg constructor, so the propagator cannot arrive as a constructor
	// parameter: a bean installs it into the package handle below instead.
	// Exported as a gs.Rooter so gs instantiates it even though nothing injects
	// it — without a collected-type export an unreachable bean is never created
	// and the filter would keep reading go-spring's default.
	gs.Provide(newPropagatorHook, gs.IndexArg(0, gs.TagArg("?"))).
		Export(gs.As[gs.Rooter]()).Caller(1)
}

const loadTestFilterKey = "loadtest"

// propagator is the application's load-test convention, installed once at wiring
// time by newPropagatorHook; nil means go-spring's default
// ([traffic.NewDefaultPropagator]). A handle rather than a filter field for the
// same reason the fault filter keeps one: dubbo, not this starter, constructs
// the filter instance.
var propagator atomic.Pointer[traffic.Propagator]

// propagatorHook is the marker bean whose construction installs the propagator.
type propagatorHook struct{}

func newPropagatorHook(p traffic.Propagator) (*propagatorHook, error) {
	if p == nil {
		propagator.Store(nil)
		return &propagatorHook{}, nil
	}
	propagator.Store(&p)
	return &propagatorHook{}, nil
}

// loadTestPropagator returns the installed convention, or go-spring's default
// when the container provided none. Read per call so a late installation is
// visible without restarting.
func loadTestPropagator() traffic.Propagator {
	if p := propagator.Load(); p != nil {
		return *p
	}
	// DefaultBinding is complete, so this cannot fail.
	p, _ := traffic.NewDefaultPropagator(traffic.DefaultBinding())
	return p
}

type loadTestFilter struct{}

func newLoadTestFilter() filter.Filter { return &loadTestFilter{} }

// Invoke tags the call context as load-test traffic when the dubbo attachment
// carried by the invocation has the marker key. It is the dubbo inbound
// companion to cloud/governance/traffic's outbound carrier injection. The attachment value
// decodes as string or []byte depending on the protocol; both are handled.
func (f *loadTestFilter) Invoke(ctx context.Context, invoker base.Invoker, inv base.Invocation) result.Result {
	if att := inv.Attachments(); att != nil {
		ctx = loadTestPropagator().Extract(ctx, attachmentCarrier(att))
	}
	return invoker.Invoke(ctx, inv)
}

// attachmentCarrier adapts a dubbo attachment map to
// [propagate.Carrier]. An attachment value decodes as string under the triple
// protocol and []byte under some others; both spellings read, any other type
// reads as absent. A view: Set writes through.
type attachmentCarrier map[string]any

var _ propagate.Carrier = attachmentCarrier(nil)

// Keys returns the map's keys, in no particular order.
func (a attachmentCarrier) Keys() []string {
	keys := make([]string, 0, len(a))
	for k := range a {
		keys = append(keys, k)
	}
	return keys
}

// Values returns the string or []byte value stored under key, nil otherwise.
func (a attachmentCarrier) Values(key string) []string {
	switch v := a[key].(type) {
	case string:
		if v != "" {
			return []string{v}
		}
	case []byte:
		if len(v) > 0 {
			return []string{string(v)}
		}
	}
	return nil
}

// Set stores value under key as a string, replacing.
func (a attachmentCarrier) Set(key, value string) {
	a[key] = value
}

// OnResponse is a pass-through; the filter only reads inbound attachments.
func (f *loadTestFilter) OnResponse(_ context.Context, res result.Result, _ base.Invoker, _ base.Invocation) result.Result {
	return res
}
