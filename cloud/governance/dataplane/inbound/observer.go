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

// observer.go is inbound's request-plane observation handle: the narrow shape a
// middleware drives, and its standard OTel-backed implementation. The metric
// seam is [observe.Metrics], injected into the handle's construction; spans and
// the access log are standard and live inside the implementation.

package inbound

import (
	"context"

	kfault "go-spring.org/cloud/governance/capability/fault"
	"go-spring.org/cloud/governance/dataplane/inbound/observe"
)

// Observation is what a middleware may read from one request's observation: the
// response half it records while answering. The interface is deliberately
// narrow — MarkInjected is the handle's to call, not the middleware's.
type Observation interface {
	// Response returns the holder the handler fills as it answers.
	Response() *observe.Response
	// MarkInjected records the fault decision the admission stage made.
	MarkInjected(f kfault.Result)
}

// Observer emits one route's inbound signals. It mirrors the forms the handle
// offers: Run (callback) and Begin/End (split), so a protocol can drive it at
// whatever hook points it has. It is the handle's internal composition shape,
// not a replaceable seam: metrics go through [observe.Metrics].
type Observer interface {
	Run(ctx context.Context, op observe.ServerOperation, fn func(context.Context, Observation) error) error
	Begin(ctx context.Context, op observe.ServerOperation) (context.Context, Observation)
	End(ob Observation, err error)
}

// newObserver builds the standard implementation of [Observer] for one route:
// the OTel-backed [observe.ServerObserver] with its metric seam wired in.
func newObserver(system, service string, m observe.Metrics) Observer {
	return otelObserver{ServerObserver: observe.NewServerObserver(system, service, m)}
}

// otelObserver adapts [observe.ServerObserver] to the narrow [Observer] shape.
type otelObserver struct {
	*observe.ServerObserver
}

// Run implements [Observer].
func (o otelObserver) Run(ctx context.Context, op observe.ServerOperation, fn func(context.Context, Observation) error) error {
	return o.ServerObserver.Run(ctx, op, func(ctx context.Context, ob *observe.Observation) error {
		return fn(ctx, ob)
	})
}

// Begin implements [Observer].
func (o otelObserver) Begin(ctx context.Context, op observe.ServerOperation) (context.Context, Observation) {
	ctx, ob := o.ServerObserver.Begin(ctx, op)
	return ctx, ob
}

// End implements [Observer].
func (o otelObserver) End(ob Observation, err error) {
	o.ServerObserver.End(ob.(*observe.Observation), err)
}
