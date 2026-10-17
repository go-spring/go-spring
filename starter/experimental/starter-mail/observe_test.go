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

package StarterMail

import (
	"go-spring.org/cloud/chain"
	"strings"
	"testing"

	"go-spring.org/stdlib/testing/assert"
	"go.opentelemetry.io/otel/attribute"
)

// attrsLine renders attributes as "k=v" pairs in slice order.
func attrsLine(attrs []attribute.KeyValue) string {
	parts := make([]string, 0, len(attrs))
	for _, a := range attrs {
		parts = append(parts, string(a.Key)+"="+a.Value.Emit())
	}
	return strings.Join(parts, ",")
}

// TestOperationDeclaresFamilyVocabulary proves one send declares the identity the
// emitter needs: the span name, the family's metric prefix, the bounded email.*
// labels and this starter's access-log tag. The non-idempotent marker that stops
// a configured retry from sending the same mail twice now rides on the
// [chain.Operation] passed to [resilience.Run] at the [GuardMailer] call site.
func TestOperationDeclaresFamilyVocabulary(t *testing.T) {
	op := operation([]*Message{{To: []string{"alice@example.com"}, Subject: "hi"}})
	assert.String(t, op.Request.Action).Equal("mail.send")
	assert.String(t, op.Observe.Metric.Name).Equal("email.client")
	assert.String(t, attrsLine(op.Observe.Metric.Attrs)).Equal("email.system=smtp,email.operation=send")
	assert.That(t, op.Observe.Log.Tag == accessTag).True()
}

// TestOperationDetailCarriesNoPII is the privacy guard: recipients and subject
// are personal data, and a failed send writes its detail to the access log at
// Warn unconditionally — so neither may appear anywhere the operation declares.
// Asserted over Attrs and Detail together, because either one would record it.
func TestOperationDetailCarriesNoPII(t *testing.T) {
	op := operation([]*Message{{
		To:      []string{"alice@example.com", "bob@example.com"},
		Cc:      []string{"carol@example.com"},
		Subject: "daily report",
	}})
	rendered := op.Request.Action + "," + attrsLine(op.Observe.Metric.Attrs) + "," + attrsLine(detailAttrs(op))
	for _, pii := range []string{"alice@example.com", "bob@example.com", "carol@example.com", "daily report"} {
		assert.That(t, strings.Contains(rendered, pii)).False()
	}
}

// TestOperationCountsEveryRecipient proves the detail still says how big the
// batch was, aggregating To and Cc across every message of the call.
func TestOperationCountsEveryRecipient(t *testing.T) {
	op := operation([]*Message{
		{To: []string{"alice@example.com"}, Cc: []string{"carol@example.com"}},
		{To: []string{"bob@example.com"}},
	})
	assert.String(t, attrsLine(detailAttrs(op))).Equal("email.recipients.count=3")
}

// detailAttrs returns the operation's span attributes that are NOT metric
// labels — the per-call detail the old Call.Detail field held, reconstructed
// from the split destination lists.
func detailAttrs(op chain.Operation) []attribute.KeyValue {
	labeled := make(map[attribute.Key]bool, len(op.Observe.Metric.Attrs))
	for _, a := range op.Observe.Metric.Attrs {
		labeled[a.Key] = true
	}
	var out []attribute.KeyValue
	for _, a := range op.Observe.Span.Attrs {
		if !labeled[a.Key] {
			out = append(out, a)
		}
	}
	return out
}
