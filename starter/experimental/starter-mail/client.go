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

// client.go is the "resource entity" concept of this starter: the Mailer
// wrapper mail beans are injected as, hollowed into an InnerMailer chain — the
// identity layer declares each send, the governance layer runs it under the
// resilience executor, the adapter layer builds and delivers the messages.

package StarterMail

import (
	"bytes"
	"context"

	"github.com/wneessen/go-mail"
	"go-spring.org/cloud"
	"go-spring.org/cloud/chain"
	"go-spring.org/cloud/observability"
	"go-spring.org/cloud/resilience"
	"go-spring.org/stdlib/errutil"
)

// Attachment is a file attached to a Message. Filename is the name shown to the
// recipient; Data holds the raw file bytes. The mailer does not read from disk —
// the caller renders or loads the content and passes the bytes in.
type Attachment struct {
	Filename string
	Data     []byte
}

// Message describes one outbound email. A plain-text body, an HTML body, or both
// (sent as a multipart/alternative) may be supplied; the recipient's client
// picks the richest part it can render. The starter intentionally ships no
// template engine — the caller renders HTML itself and passes the final string.
type Message struct {
	// From overrides the mailer's default sender for this message. When empty
	// the mailer's configured From is used; if both are empty, Send fails.
	From string

	// To, Cc, and Bcc are recipient address lists. At least one To is required.
	To  []string
	Cc  []string
	Bcc []string

	// Subject is the email subject line.
	Subject string

	// Text is the plain-text body. HTML is the HTML body. When both are set the
	// message is multipart/alternative; when only one is set that is the body.
	Text string
	HTML string

	// Attachments are files attached to the message.
	Attachments []Attachment
}

// Mailer wraps an SMTP client configured for one server. It is safe to hold as a
// bean: each Send dials the server, delivers, and closes the connection, so no
// long-lived socket is kept between calls. It holds exactly two things: the
// embedded [InnerMailer] chain head Send runs through — reorganized by wrapping
// the head in a layer of your own — and [Mailer.Client], the raw client as a
// read-only handle. [newMailer] is the only way to build one.
type Mailer struct {
	// The embedded InnerMailer is the chain Send runs through: identity over
	// governance over the raw adapter by default, so the default path is always
	// declared and protected. Reorganize it by wrapping the head (see
	// [InnerMailer]); build-time only.
	InnerMailer

	// Client is the raw go-mail client — the original object, not a wrapper. A
	// read-only handle, never to send through: that would bypass the chain.
	Client *mail.Client

	// from is the default sender, fixed by [newMailer].
	from string
}

// Send delivers one or more messages. It opens a single connection, sends all of
// them, and closes it. An error is returned if any message is invalid or if the
// connection or delivery fails. Declared, governed and observed through the
// chain; see [InnerMailer].
func (m *Mailer) Send(ctx context.Context, msgs ...*Message) error {
	if len(msgs) == 0 {
		return nil
	}
	return m.InnerMailer.Send(ctx, msgs...)
}

// Close tears the mailer down through its chain — the head's Release(true):
// the executor goes, and with it nothing else, because the SMTP client holds no
// live resource (each Send dials and closes its own connection). It is the gs
// destroy method.
func (m *Mailer) Close() error { return m.InnerMailer.Release(true) }

// InnerMailer is the seam the mailer's internals are hollowed into: the send
// traffic SMTP carries. go-mail offers no hook or plugin point and delivers a
// concrete type, so this interface is the ONLY way to modify what happens under
// the promoted Send.
//
// The default chain is the identity layer over the governance layer over a raw
// adapter, and the embedded InnerMailer is where a custom layer goes: implement
// this interface (embed the head you found to inherit the methods you do not
// care about), then assign your layer over it. The chain under the layer keeps
// doing its job — the batch a layer rewrites is the batch the identity layer
// declares, and the executor still protects every send.
//
// Release follows the chain protocol: every layer takes away its OWN resources
// and passes the flag to the layer under it; [Mailer.Close] is the head's
// Release(true).
type InnerMailer interface {
	// Send delivers the message batch in one connection.
	Send(ctx context.Context, msgs ...*Message) error
	// Release releases the layer's own resources, then hands releaseRaw to
	// the layer under it.
	Release(releaseRaw bool) error
}

// RawMailer is the adapter layer at the tail: it builds every message and hands
// the batch to the client in one connection. The build/deliver logic lives here
// — the layers above it know nothing of MIME. [NewRawMailer] builds it.
type RawMailer struct {
	client *mail.Client
	from   string
}

// NewRawMailer wraps a raw client (with its default sender) as the chain's
// tail.
func NewRawMailer(client *mail.Client, from string) *RawMailer {
	return &RawMailer{client: client, from: from}
}

// Release closes the client's idle connection, if any — with releaseRaw only;
// the shallow release leaves it as-is. Each Send dials its own connection, so
// there is usually nothing live to release.
func (r *RawMailer) Release(releaseRaw bool) error {
	if !releaseRaw {
		return nil
	}
	return r.client.Close()
}

// Send builds every message, then hands the batch to the client in one
// connection.
func (r *RawMailer) Send(ctx context.Context, msgs ...*Message) error {
	built := make([]*mail.Msg, 0, len(msgs))
	for _, msg := range msgs {
		mm, err := r.build(msg)
		if err != nil {
			return err
		}
		built = append(built, mm)
	}
	if err := r.client.DialAndSendWithContext(ctx, built...); err != nil {
		return errutil.Explain(err, "mail: send failed")
	}
	return nil
}

// build converts a Message into a go-mail Msg, validating the sender and
// recipients and assembling the body and attachments.
func (r *RawMailer) build(msg *Message) (*mail.Msg, error) {
	from := msg.From
	if from == "" {
		from = r.from
	}
	if from == "" {
		return nil, errutil.Explain(nil, "mail: no From address (set message.From or spring.mail.instances...from)")
	}
	if len(msg.To) == 0 {
		return nil, errutil.Explain(nil, "mail: message has no recipients (To)")
	}

	mm := mail.NewMsg()
	if err := mm.From(from); err != nil {
		return nil, errutil.Explain(err, "mail: invalid From address %q", from)
	}
	if err := mm.To(msg.To...); err != nil {
		return nil, errutil.Explain(err, "mail: invalid To address")
	}
	if len(msg.Cc) > 0 {
		if err := mm.Cc(msg.Cc...); err != nil {
			return nil, errutil.Explain(err, "mail: invalid Cc address")
		}
	}
	if len(msg.Bcc) > 0 {
		if err := mm.Bcc(msg.Bcc...); err != nil {
			return nil, errutil.Explain(err, "mail: invalid Bcc address")
		}
	}
	mm.Subject(msg.Subject)

	switch {
	case msg.HTML != "" && msg.Text != "":
		mm.SetBodyString(mail.TypeTextPlain, msg.Text)
		mm.AddAlternativeString(mail.TypeTextHTML, msg.HTML)
	case msg.HTML != "":
		mm.SetBodyString(mail.TypeTextHTML, msg.HTML)
	default:
		mm.SetBodyString(mail.TypeTextPlain, msg.Text)
	}

	for _, a := range msg.Attachments {
		if err := mm.AttachReader(a.Filename, bytes.NewReader(a.Data)); err != nil {
			return nil, errutil.Explain(err, "mail: failed to attach %q", a.Filename)
		}
	}
	return mm, nil
}

// GuardMailer is the governance layer: it runs every send under the resilience
// executor, which applies rate limiting, breaking and retrying — and emits the
// send's span, metrics and access log from the one point that sees the whole
// call, attempts included. [NewGuardMailer] builds it, executor included: the
// executor is the layer's own business end to end — built, used and closed
// inside it.
type GuardMailer struct {
	exec chain.Executor
	next InnerMailer
}

// NewGuardMailer builds the governance layer over next, constructing its own
// executor: params carries the container's facilities (see [cloud.ClientParams])
// and instanceName/host fix the governance label its limiter and breaker state
// scope by.
func NewGuardMailer(next InnerMailer, instanceName, host string, params cloud.ClientParams) *GuardMailer {
	label := resilience.ServiceLabel("mail", instanceName, host)
	return &GuardMailer{exec: params.ExecutorFor("mail", label), next: next}
}

// Release hands releaseRaw to the layer under it, and on the FULL teardown
// takes down the executor — the layer's own resource.
func (g *GuardMailer) Release(releaseRaw bool) error {
	err := g.next.Release(releaseRaw)
	if releaseRaw && g.exec != nil {
		_ = g.exec.Close()
	}
	return err
}

// Send runs the batch under the executor with retry.
func (g *GuardMailer) Send(ctx context.Context, msgs ...*Message) error {
	_, err := resilience.Run(ctx, g.exec, func(attemptCtx context.Context) (struct{}, error) {
		return struct{}{}, g.next.Send(attemptCtx, msgs...)
	})
	return err
}

// ObsMailer is the identity layer at the head: it names the batch (see
// observe.go) and hands the context down. It emits nothing itself — emission
// happens in the governance layer under it, the one point that sees the whole
// call. [NewObsMailer] builds it.
type ObsMailer struct {
	next InnerMailer
}

// NewObsMailer builds the identity layer over next.
func NewObsMailer(next InnerMailer) *ObsMailer { return &ObsMailer{next: next} }

// Release hands releaseRaw to the layer under it — this layer holds no
// resource.
func (o *ObsMailer) Release(releaseRaw bool) error { return o.next.Release(releaseRaw) }

// Send declares the batch's semantic identity, then hands it down.
func (o *ObsMailer) Send(ctx context.Context, msgs ...*Message) error {
	return o.next.Send(observability.WithOperation(ctx, operation(msgs)), msgs...)
}
