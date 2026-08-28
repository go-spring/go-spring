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

package StarterNats

import (
	"testing"

	"github.com/nats-io/nats.go"
	"go-spring.org/cloud/experimental/messaging"
)

func TestToNatsHeader(t *testing.T) {
	if h := toNatsHeader(nil); h != nil {
		t.Fatalf("empty envelope headers must map to nil, got %v", h)
	}
	h := toNatsHeader(map[string]string{"a": "1"})
	if h.Get("a") != "1" {
		t.Fatalf("want a=1, got %q", h.Get("a"))
	}
}

func TestFromNatsMsgKeyRoundTrip(t *testing.T) {
	// Publish side: msg.Key rides the reserved x-msg-key NATS header.
	msg := &messaging.Message{Key: "order-42", Payload: []byte("body")}
	nm := &nats.Msg{Subject: "s", Data: msg.Payload, Header: toNatsHeader(msg.Headers)}
	if msg.Key != "" {
		if nm.Header == nil {
			nm.Header = nats.Header{}
		}
		nm.Header.Set(headerMsgKey, msg.Key)
	}
	got := fromNatsMsg(nm)
	if got.Key != "order-42" {
		t.Fatalf("want key order-42, got %q", got.Key)
	}
	if _, ok := got.Headers[headerMsgKey]; ok {
		t.Fatal("x-msg-key is transport-internal and must not surface in Headers")
	}
	if string(got.Payload) != "body" {
		t.Fatalf("want body, got %q", got.Payload)
	}
}

func TestFromNatsMsgMultiValueHeaderKeepsFirst(t *testing.T) {
	nm := &nats.Msg{Subject: "s", Data: []byte("x"), Header: nats.Header{}}
	nm.Header.Add("h", "first")
	nm.Header.Add("h", "second")
	got := fromNatsMsg(nm)
	if got.Headers["h"] != "first" {
		t.Fatalf("want first value, got %q", got.Headers["h"])
	}
}
