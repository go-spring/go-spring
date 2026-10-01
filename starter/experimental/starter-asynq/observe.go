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

// observe.go declares what an asynq enqueue IS. The signals themselves — the
// span, the duration metrics, the access log — are emitted by the resilience
// layer, the one place on the executor chain that sees a whole call (retries
// included). This file therefore holds no emission code: only the vocabulary
// that this starter alone knows, because only it knows these calls reach a task
// queue.
package StarterAsynq

import (
	"go-spring.org/cloud/observability"
	"go-spring.org/stdlib/strutil"

	"go-spring.org/log"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
)

// asynqSystem is the value the family's messaging.system label carries for this
// backend — the family's shared vocabulary, not a per-file choice.
const asynqSystem = "asynq"

// maxDestination bounds the task type captured as messaging.destination.name. A
// task type can be long and a span or a log line has no use for all of it.
const maxDestination = 512

// accessTag is the static log tag for the Asynq access log. It is registered
// here, at package init, because a tag must exist before the framework's first
// property refresh — see [log.RegisterTag].
var accessTag = log.RegisterAppTag("asynq", "access")

// operation is the semantic identity of one enqueue call.
//
// The task type rides in Detail rather than Attrs: task types are drawn from an
// open set, so as a metric label one would multiply the series without bound.
// Detail reaches the span and the log — where the destination is exactly what
// makes a line worth reading — and never a label. An empty task type carries no
// detail at all, which is also what levelled its success log at Info rather than
// Debug.
func operation(op, taskType string) observability.Operation {
	o := observability.Operation{
		Name:   op,
		Metric: "messaging.client",
		Attrs: []attribute.KeyValue{
			attribute.String("messaging.system", asynqSystem),
			attribute.String("messaging.operation", op),
		},
		LogTag: accessTag,
		// An enqueue is the producer edge of the trace: the task is handed to a
		// broker for a worker to consume later, and the edge is what keeps that
		// visible once the emitter opens the span.
		SpanKind: trace.SpanKindProducer,
	}
	if taskType != "" {
		o.Detail = []attribute.KeyValue{
			attribute.String("messaging.destination.name", strutil.Truncate(taskType, maxDestination)),
		}
	}
	return o
}
