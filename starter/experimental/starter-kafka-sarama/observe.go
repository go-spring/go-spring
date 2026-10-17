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
// executor chain that sees a whole call (retries included). This file therefore
// holds no per-call emission code: only the vocabulary this starter alone
// knows, because only it knows these calls reach a broker.
//
// Nothing here is kept outside the declared operation: this starter has no
// connection-state counter, no observable gauge, no pool-stat instrument, so
// there is no non-per-call signal left to own. (The sarama logging bridge in
// logger.go is not an instrument — it routes sarama's own connection events
// into go-spring's log — so it stays, but it emits no observation.)

package StarterKafkaSarama

import (
	"go-spring.org/cloud/runtime/dataplane/chain"
	"go-spring.org/log"
)

// accessTag is the static log tag for the kafka access log. It is registered
// here, at package init, because a tag must exist before the framework's first
// property refresh — see [log.RegisterTag].
var accessTag = log.RegisterAppTag("kafka", "access")

// kafkaSystem is the value the family's messaging.system label carries for this
// backend — the family's shared vocabulary, not a per-file choice.
const kafkaSystem = "kafka"

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
// [cloud/runtime/dataplane/chain]. This starter supplies only what only it knows: that the calls
// reach a kafka broker, and their word and destination.
func operation(word opWord, topic string) chain.Operation {
	op := chain.MessagingOp(word.shape, kafkaSystem, word.word, topic)
	op.Observe.Log.Tag = accessTag
	return op
}
