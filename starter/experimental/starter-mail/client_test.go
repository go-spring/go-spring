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
	"context"
	"testing"

	"go-spring.org/cloud"
	"go-spring.org/stdlib/testing/assert"
)

// fakeMailer is a scripted tail: it records the batches it was handed. It
// stands in for the raw adapter so chain tests need no SMTP server.
type fakeMailer struct {
	batches  [][]*Message
	released bool
}

func (f *fakeMailer) Send(_ context.Context, msgs ...*Message) error {
	f.batches = append(f.batches, msgs)
	return nil
}

func (f *fakeMailer) Release(releaseRaw bool) error {
	if releaseRaw {
		f.released = true
	}
	return nil
}

// bccLayer wraps the head it found and adds a fixed Bcc to every message — the
// kind of behavior change no SMTP hook could express.
type bccLayer struct {
	InnerMailer
	bcc string
}

func (b bccLayer) Send(ctx context.Context, msgs ...*Message) error {
	for _, m := range msgs {
		m.Bcc = append(m.Bcc, b.bcc)
	}
	return b.InnerMailer.Send(ctx, msgs...)
}

// newChainMailer builds a Mailer whose chain bottoms out in tail — identity
// and governance layers real (executor unmanaged, governance off).
func newChainMailer(t *testing.T, tail InnerMailer) *Mailer {
	t.Helper()
	raw := NewRawMailer(nil, "")
	guard := NewGuardMailer(raw, "hot", "", cloud.ClientParams{})
	return &Mailer{InnerMailer: NewObsMailer(&GuardMailer{exec: guard.exec, next: tail})}
}

// TestInnerChainRewritesMessages pins the wrap-head protocol: a custom layer
// over the chain head rewrites the batch, and Send runs through it.
func TestInnerChainRewritesMessages(t *testing.T) {
	fake := &fakeMailer{}
	m := newChainMailer(t, fake)
	m.InnerMailer = bccLayer{InnerMailer: m.InnerMailer, bcc: "audit@example.com"}

	msg := &Message{From: "a@example.com", To: []string{"b@example.com"}, Subject: "s"}
	assert.Error(t, m.Send(context.Background(), msg)).Nil()

	got := fake.batches[0][0]
	assert.That(t, len(got.Bcc)).Equal(1)
	assert.That(t, got.Bcc[0]).Equal("audit@example.com")
}

// TestChainRelease pins the teardown: Close is the head's Release(true) and
// reaches the tail through the layers.
func TestChainRelease(t *testing.T) {
	fake := &fakeMailer{}
	m := newChainMailer(t, fake)
	assert.Error(t, m.Close()).Nil()
	assert.That(t, fake.released).True()
}
