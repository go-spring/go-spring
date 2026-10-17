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
	"go-spring.org/cloud/runtime/dataplane/chain"
	"go-spring.org/log"
)

// asynqSystem is the value the family's messaging.system label carries for this
// backend — the family's shared vocabulary, not a per-file choice.
const asynqSystem = "asynq"

// accessTag is the static log tag for the Asynq access log. It is registered
// here, at package init, because a tag must exist before the framework's first
// property refresh — see [log.RegisterTag].
var accessTag = log.RegisterAppTag("asynq", "access")

// operation is the semantic identity of one enqueue call. An enqueue is a
// publish — the task is handed to a broker for a worker to consume later — so
// the messaging family's vocabulary in [wire] describes it; this starter
// supplies only the system and the task type.
func operation(op, taskType string) chain.Operation {
	o := chain.MessagingOp(chain.KindPublish, asynqSystem, op, taskType)
	o.Observe.Log.Tag = accessTag
	return o
}
