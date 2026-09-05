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

package StarterMQTT

import (
	"context"
	"testing"
)

// TestSpanHelpersNoOpWithoutOTel verifies the span helpers run against the
// no-op OTel globals: StartPublishSpan/StartConsumeSpan build the local
// observers lazily and End records the outcome without panicking, so a blank
// import of this starter is safe without starter-otel.
func TestSpanHelpersNoOpWithoutOTel(t *testing.T) {
	ctx := context.Background()
	_, sp := StartPublishSpan(ctx, "sensors/temp")
	if sp == nil {
		t.Fatal("publish span should not be nil")
	}
	sp.End(nil)

	_, sp2 := StartConsumeSpan(ctx, fakeMessage("sensors/temp"))
	if sp2 == nil {
		t.Fatal("consume span should not be nil")
	}
	sp2.End(context.Canceled)
}

// fakeMessage is a minimal mqtt.Message carrying only the topic.
type fakeMessage string

func (m fakeMessage) Duplicate() bool   { return false }
func (m fakeMessage) Qos() byte         { return 0 }
func (m fakeMessage) Retained() bool    { return false }
func (m fakeMessage) Topic() string     { return string(m) }
func (m fakeMessage) MessageID() uint16 { return 0 }
func (m fakeMessage) Payload() []byte   { return nil }
func (m fakeMessage) Ack()              {}
