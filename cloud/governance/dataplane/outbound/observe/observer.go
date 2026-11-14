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

package observe

import (
	"context"

	"go-spring.org/cloud/governance/dataplane/outbound/chain"
)

// OperationObserver is the self-emission entry for a client whose transport
// offers no seam for the resilience executor — a poll-driven subscriber, a
// message producer whose hook is call-site-only — but which still wants the
// standard client signals. It emits exactly what [WrapClientExecutor] emits (the
// same span, metric names, labels and access log), and applies NO governance:
// there is no rate limit, circuit breaker, retry or timeout in this path.
//
// A client that has a seam for the executor declares its operation and routes
// through it instead; this exists so a client that cannot is not left
// unobserved.
type OperationObserver struct {
	*callEmitter
}

// NewOperationObserver builds the observer for one client system/service. system
// is the backend name ("kafka", "webhook"); service is the per-instance label
// (see [label.ServiceLabel]).
func NewOperationObserver(system, service string, m Metrics) *OperationObserver {
	return &OperationObserver{callEmitter: &callEmitter{
		system:  system,
		service: service,
		metrics: m,
	}}
}

// Observe runs fn as one observed call, emitting the call span, the
// call-level duration, the in-flight gauge, the attempt-level duration and the
// access log. op is the operation's description (see [chain.Operation]); fn's
// error is returned unchanged — this is instrumentation, not error policy.
//
// rec is the same explicit record [chain.Executor.Execute] takes, for the same
// reason: whoever owns the call owns its record. It must be non-nil — a caller
// that wants only the signals passes a record it simply never reads.
func (o *OperationObserver) Observe(ctx context.Context, op chain.Operation, rec *chain.Recorder, fn func(context.Context) error) error {
	return o.call(ctx, op, rec, fn, true)
}
