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
	"bytes"
	"context"
	"errors"
	"strings"
	"testing"

	mqtt "github.com/eclipse/paho.mqtt.golang"
	"go-spring.org/cloud/messaging"
	"go-spring.org/log"
)

// fieldExecutor runs the handler under a context it annotates, standing in for
// an executor that opens the attempt's span: whatever rode on the context the
// handler ran under is what the driver's outcome line has to carry.
type fieldExecutor struct{}

func (fieldExecutor) Execute(ctx context.Context, fn func(context.Context) error) error {
	return fn(log.WithFields(ctx, log.String("attempt", "under-the-executor")))
}

func (fieldExecutor) Close() error { return nil }

// callbackClient captures the paho callback Subscribe installs, so a test can
// deliver one message without a broker.
type callbackClient struct {
	mqtt.Client
	onMessage mqtt.MessageHandler
}

func (c *callbackClient) Subscribe(_ string, _ byte, cb mqtt.MessageHandler) mqtt.Token {
	c.onMessage = cb
	return &fakeToken{}
}

// TestHandlerFailureJoinsTheAttemptContext pins the driver's failure line to the
// context its handler ran under: the attempt's identity (a field an executor put
// on it here, the attempt's span in production) reaches the line, instead of the
// line sitting outside that context on a bare background one.
func TestHandlerFailureJoinsTheAttemptContext(t *testing.T) {
	cl := &callbackClient{}
	clientGuards.Store(cl, &clientGuard{exec: fieldExecutor{}, serviceLabel: "mqtt:test"})
	defer clientGuards.Delete(cl)

	sub := &subscriber{cl: cl, topic: "t"}
	err := sub.Subscribe(context.Background(), func(context.Context, *messaging.Message) error {
		return errors.New("handler boom")
	})
	if err != nil {
		t.Fatal(err)
	}
	if cl.onMessage == nil {
		t.Fatal("Subscribe must install a message callback")
	}

	prev := log.Stdout
	buf := bytes.NewBuffer(nil)
	log.Stdout = buf
	defer func() { log.Stdout = prev }()

	cl.onMessage(cl, fakeMessage("t"))

	out := buf.String()
	if !strings.Contains(out, "mqtt driver handler failed") {
		t.Fatalf("expected the driver's failure line, got: %s", out)
	}
	if !strings.Contains(out, "attempt=under-the-executor") {
		t.Fatalf("the failure line must carry the attempt's context, got: %s", out)
	}
}
