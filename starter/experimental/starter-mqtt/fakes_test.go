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
	"sync/atomic"
	"testing"
	"time"

	mqtt "github.com/eclipse/paho.mqtt.golang"
)

// fakeToken is a completed paho token.
type fakeToken struct{ err error }

func (t *fakeToken) Wait() bool                     { return true }
func (t *fakeToken) WaitTimeout(time.Duration) bool { return true }
func (t *fakeToken) Error() error                   { return t.err }
func (t *fakeToken) Done() <-chan struct{}          { ch := make(chan struct{}); close(ch); return ch }

// fakeMQTTClient records whether the underlying Publish ran.
type fakeMQTTClient struct {
	mqtt.Client
	published atomic.Int32
}

func (f *fakeMQTTClient) Publish(topic string, qos byte, retained bool, payload interface{}) mqtt.Token {
	f.published.Add(1)
	return &fakeToken{}
}

// TestAttachObserverInstallsObserver pins that AttachObserver indexes an observer
// for the client — the driver calls it while building the client, so a client is
// never left unobserved — and that forgetObserver drops it again.
func TestAttachObserverInstallsObserver(t *testing.T) {
	cl := &fakeMQTTClient{}

	AttachObserver(cl, "tcp://127.0.0.1:1883", testProp)
	if _, ok := clientGuards.Load(cl); !ok {
		t.Fatal("AttachObserver must install an observer")
	}
	forgetObserver(cl)
	if _, ok := clientGuards.Load(cl); ok {
		t.Fatal("forgetObserver must drop the observer")
	}
}
