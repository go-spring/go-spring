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
	"context"
	"errors"
	"testing"
	"time"

	"github.com/nats-io/nats.go"
	"go-spring.org/stdlib/testing/assert"
)

// fakeTail is a scripted InnerConn tail: it counts the calls that reached it
// and answers from a settable error. It stands in for the raw adapter so chain
// tests need no live nats server.
type fakeTail struct {
	ran int
	err error
}

func (f *fakeTail) PublishMsg(msg *nats.Msg) error {
	f.ran++
	return f.err
}

func (f *fakeTail) PublishMsgContext(ctx context.Context, msg *nats.Msg) error {
	f.ran++
	return f.err
}

func (f *fakeTail) RequestGuarded(context.Context, string, []byte, time.Duration) (*nats.Msg, error) {
	f.ran++
	return nil, f.err
}

func (f *fakeTail) Release(bool) error { return nil }

// TestChainPassThrough proves the no-observer stance: a chain with no observer
// attached runs the call inline and returns its result unchanged.
func TestChainPassThrough(t *testing.T) {
	tail := &fakeTail{}
	c := &Conn{InnerConn: NewObsConn(tail, nil, testProp)}

	boom := errors.New("boom")
	tail.err = boom
	assert.Error(t, c.PublishMsgContext(context.Background(), &nats.Msg{Subject: "t.s"})).Is(boom)
	assert.That(t, tail.ran).Equal(1)
}
