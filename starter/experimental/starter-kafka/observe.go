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

// observe.go declares what a kafka publish or consume IS. The signals
// themselves — the span, the duration metrics (call-level and attempt-level),
// the access log — are emitted by the resilience layer, the one place on the
// executor chain that sees a whole call. This file therefore holds no emission
// code: only the vocabulary this starter alone knows, because only it knows
// these calls reach a broker.
//
// What is NOT declared here is the broker/client-level telemetry kotel installs
// on the franz-go client (messaging.kafka.* connections, bytes, per-node
// health). That is third-party instrumentation this starter merely enables, not
// a per-call signal, so it stays where kotel puts it.

package StarterKafka

import (
	"go-spring.org/cloud/chain"
	"go-spring.org/log"
)

// kafkaSystem is the value the family's messaging.system label carries for this
// backend — the family's shared vocabulary, not a per-file choice.
const kafkaSystem = "kafka"

// accessTag is the static log tag for the kafka access log. It is registered
// here, at package init, because a tag must exist before the framework's first
// property refresh — see [log.RegisterTag].
var accessTag = log.RegisterAppTag("messaging", "access")

// opWord is one messaging operation this starter reports: the OTel operation
// word — carried by `messaging.operation`, the span name and the access log —
// paired with the interaction shape it projects onto, which picks the trace
// edge. Stating the two together keeps them from drifting, and makes a
// misspelled word a COMPILE error instead of a wrong metric label: the shape is
// the one that word means, not a fallback for words nobody recognized.
type opWord struct {
	word  string
	shape chain.Kind
}

var (
	// opSend is a one-way hand-off to the broker: the producer edge.
	opSend = opWord{word: "send", shape: chain.KindPublish}
	// opProcess is the handling of one delivered message: the consumer edge.
	opProcess = opWord{word: "process", shape: chain.KindConsume}
)

// operation is the semantic identity of one kafka operation: the operation
// word it reports and the destination it addresses.
//
// The vocabulary — the family's attribute keys, the bounded/open split that puts
// the destination in detail and the system in a label, and the trace edge each
// word adds — is the messaging family's, not this file's; it lives in
// [cloud/chain]. This starter supplies only what only it knows: that the calls
// reach a kafka broker, and their word and destination.
func operation(word opWord, topic string) chain.Operation {
	op := chain.MessagingOp(word.shape, kafkaSystem, word.word, topic)
	op.Observe.Log.Tag = accessTag
	return op
}
