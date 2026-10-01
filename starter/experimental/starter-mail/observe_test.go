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
// labels, this starter's access-log tag — and the non-idempotent marker that
// stops a configured retry from sending the same mail twice.
func TestOperationDeclaresFamilyVocabulary(t *testing.T) {
	op := operation([]*Message{{To: []string{"alice@example.com"}, Subject: "hi"}})
	assert.String(t, op.Name).Equal("mail.send")
	assert.String(t, op.Metric).Equal("email.client")
	assert.String(t, attrsLine(op.Attrs)).Equal("email.system=smtp,email.operation=send")
	assert.That(t, op.LogTag == accessTag).True()
	assert.That(t, op.NonIdempotent).True()
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
	rendered := op.Name + "," + attrsLine(op.Attrs) + "," + attrsLine(op.Detail)
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
	assert.String(t, attrsLine(op.Detail)).Equal("email.recipients.count=3")
}
