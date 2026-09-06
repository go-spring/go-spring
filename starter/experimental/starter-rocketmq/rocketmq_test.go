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

package StarterRocketmq

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/apache/rocketmq-client-go/v2/primitive"
	"go-spring.org/cloud/governance/resilience"
	"go-spring.org/cloud/governance/traffic/canonical"
	"go-spring.org/stdlib/testing/assert"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/trace"
)

// newClientWithPolicy builds a Client whose executor is wired to a real
// executor from the default resilience driver. The tests never dial a name
// server — they drive execute directly with a stubbed call.
func newClientWithPolicy(t *testing.T, p resilience.Policy) *Client {
	d, err := resilience.GetDriver("default")
	assert.Error(t, err).Nil()
	exec, err := d.NewExecutor(p)
	assert.Error(t, err).Nil()
	return &Client{exec: exec, resource: "rocketmq:test"}
}

// TestExecutePassThrough proves the zero-config opt-in: a Client with no
// executor attached runs the call inline and returns its result unchanged.
func TestExecutePassThrough(t *testing.T) {
	cl := &Client{}
	boom := errors.New("boom")

	assert.Error(t, cl.execute(context.Background(), func(context.Context) error { return nil })).Nil()
	assert.Error(t, cl.execute(context.Background(), func(context.Context) error { return boom })).Is(boom)
}

// TestExecuteRateLimit confirms the flow-control path: once the burst is spent,
// further calls are rejected without invoking the stub.
func TestExecuteRateLimit(t *testing.T) {
	cl := newClientWithPolicy(t, resilience.Policy{RateLimit: 1, Burst: 2})
	var ran int
	stub := func(context.Context) error {
		ran++
		return nil
	}

	assert.Error(t, cl.execute(context.Background(), stub)).Nil()
	assert.Error(t, cl.execute(context.Background(), stub)).Nil()
	assert.Error(t, cl.execute(context.Background(), stub)).Is(resilience.ErrRateLimited)
	assert.That(t, ran).Equal(2) // the rejected call never reached the stub
}

// TestMsgCarrierRoundTrip proves the OTel carrier adapter over user
// properties: a value injected through the global propagator comes back out
// on a fresh message read, which is what links producer and consumer spans.
func TestMsgCarrierRoundTrip(t *testing.T) {
	otel.SetTextMapPropagator(propagation.TraceContext{})

	out := primitive.NewMessage("t", []byte("p"))
	ctx := otel.GetTextMapPropagator().Extract(context.Background(), propagation.MapCarrier{
		"traceparent": "00-0af7651916cd43dd8448eb211c80319c-b7ad6b7169203331-01",
	})
	otel.GetTextMapPropagator().Inject(ctx, msgCarrier{out})
	tp := out.GetProperty("traceparent")
	assert.That(t, tp != "").True()
	assert.That(t, strings.Contains(tp, "0af7651916cd43dd8448eb211c80319c")).True()
	var hasKey bool
	keys := msgCarrier{out}.Keys()
	for _, k := range keys {
		if k == "traceparent" {
			hasKey = true
		}
	}
	assert.That(t, hasKey).True()

	in := primitive.NewMessage("t", nil)
	in.WithProperty("traceparent", tp)
	got := otel.GetTextMapPropagator().Extract(context.Background(), msgCarrier{in})
	assert.That(t, trace.SpanContextFromContext(got).TraceID().String()).Equal("0af7651916cd43dd8448eb211c80319c")
}

// TestFromMessageExt covers the envelope mapping, including the load-test
// marker header the driver propagates through user properties.
func TestFromMessageExt(t *testing.T) {
	ext := &primitive.MessageExt{}
	ext.Topic = "hello"
	ext.Body = []byte("value")
	ext.StoreTimestamp = 1700000000000
	ext.WithKeys([]string{"k1"})
	ext.WithProperty("from", "example")
	ext.WithProperty(canonical.MetaKeyLoadTest, "1")

	msg := fromMessageExt(ext)
	assert.That(t, msg.Key).Equal("k1")
	assert.That(t, string(msg.Payload)).Equal("value")
	assert.That(t, msg.Headers["from"]).Equal("example")
	assert.That(t, msg.Headers[canonical.MetaKeyLoadTest]).Equal("1")
	assert.That(t, msg.Timestamp.UnixMilli()).Equal(int64(1700000000000))
}
