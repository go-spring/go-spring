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

package messaging

import (
	"context"
	"errors"
	"strings"
	"testing"
)

// TestSafeHandlerPassThrough proves a clean handler is invoked unchanged:
// same message, same error.
func TestSafeHandlerPassThrough(t *testing.T) {
	boom := errors.New("handler error")
	var got *Message
	err := Recover(func(_ context.Context, msg *Message) error {
		got = msg
		return boom
	})(context.Background(), &Message{Payload: []byte("p")})
	if err != boom {
		t.Fatalf("error must pass through, got %v", err)
	}
	if string(got.Payload) != "p" {
		t.Fatalf("message must reach the handler, got %v", got)
	}
}

// TestSafeHandlerConvertsPanic proves the protection the binders rely on: a
// panicking handler surfaces as an error (the binder's normal nack/redelivery
// path) instead of unwinding into the SDK's delivery goroutine.
func TestSafeHandlerConvertsPanic(t *testing.T) {
	err := Recover(func(context.Context, *Message) error {
		panic("poison message")
	})(context.Background(), &Message{})
	if err == nil {
		t.Fatal("panic must convert to an error")
	}
	if !strings.Contains(err.Error(), "poison message") {
		t.Fatalf("error should name the panic value: %v", err)
	}
}
