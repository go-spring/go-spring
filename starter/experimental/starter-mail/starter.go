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
	"bytes"
	"context"
	"crypto/tls"
	"go-spring.org/cloud/chain"
	"go-spring.org/cloud/governance"
	"strings"

	"github.com/wneessen/go-mail"
	"go-spring.org/cloud"
	"go-spring.org/cloud/observability"
	"go-spring.org/cloud/resilience"
	"go-spring.org/log"
	"go-spring.org/spring/conf"
	"go-spring.org/spring/gs"
	"go-spring.org/stdlib/errutil"
	"go-spring.org/stdlib/flatten"
)

func init() {
	// Register one SMTP mailer bean per entry under "${spring.mail.instances}".
	// A gs.Module (rather than gs.Group) is used so each
	// instance's ctor can take the governance center alongside its config — the
	// *Mailer bean owns the resilience executor, which the ctor builds while
	// assembling the mailer and Destroy tears down — and to attach the file:line of
	// this registration to the bean for diagnostics. There is no default
	// singleton — select one by name (e.g. autowire:"notify").
	gs.Module(gs.OnProperty("spring.mail.instances"), func(r gs.BeanProvider, p flatten.Storage) error {
		// Any key an instance does not define falls back to the family-wide
		// "default" bucket: spring.mail.default.<k> is the value every
		// instance inherits unless it sets its own.
		p = flatten.WithFallback(p, "spring.mail.instances", "spring.mail.default")
		return conf.BindEach(p, "${spring.mail.instances}", func(name string, c Config) error {
			r.Provide(newMailer,
				gs.IndexArg(1, gs.ValueArg(name)),
				gs.IndexArg(2, gs.ValueArg(c)),
				// The governance center is the family's sole injection point: it hands
				// out the resilience/fault/loadbalance authorities.
			).Name(name).Destroy((*Mailer).Destroy).Caller(1)
			return nil
		})
	})
}

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
// long-lived socket is kept between calls. That is why the only resource it
// releases at shutdown, through [Mailer.Destroy], is the resilience executor.
type Mailer struct {
	client *mail.Client
	from   string

	// serviceLabel is the resilience service key ("mail:<name or host>") the
	// executor scopes limiter/breaker state by. Fixed by [newMailer].
	serviceLabel string

	// exec is the resilience executor protecting every send, set by [newMailer]
	// from the container's [cloud.ClientParams]; it is also the single emitter
	// of the send's span, metrics and access log, read off the operation [Send]
	// declares.
	exec chain.Executor
}

// newMailer builds a Mailer from config. It fails fast on a missing host or an
// unknown auth/TLS mode, and (when Ping is enabled) probes the server once at
// startup so a misconfiguration surfaces at boot rather than on the first send.
//
// center is the governance center the container injects — the family's sole
// injection point; the ctor reads the resilience and fault authorities from it
// and bundles them into the [cloud.ClientParams] it builds the mailer's executor from, so
// the mailer is assembled complete in one step. The zero bundle degrades to an
// observed-only, loudly-unmanaged executor.
func newMailer(ctx *gs.ContextProvider, name string, c Config, center *governance.Center) (*Mailer, error) {
	if err := errutil.RequireField("mail", "host", c.Host); err != nil {
		return nil, err
	}

	log.Debugf(ctx.Context, log.TagAppDef, "creating mailer host=%s port=%d auth=%s tls=%s from=%s", c.Host, c.Port, c.AuthType, c.TLS.Mode, c.From)

	opts := []mail.Option{
		mail.WithPort(c.Port),
		mail.WithTimeout(c.Timeout),
	}

	if c.Username != "" {
		authType, err := parseAuthType(c.AuthType)
		if err != nil {
			return nil, err
		}
		opts = append(opts,
			mail.WithSMTPAuth(authType),
			mail.WithUsername(c.Username),
			mail.WithPassword(c.Password),
		)
	} else {
		opts = append(opts, mail.WithSMTPAuth(mail.SMTPAuthNoAuth))
	}

	switch strings.ToLower(c.TLS.Mode) {
	case "", "starttls":
		opts = append(opts, mail.WithTLSPolicy(mail.TLSMandatory))
	case "tls", "ssl":
		opts = append(opts, mail.WithSSL())
	case "none":
		opts = append(opts, mail.WithTLSPolicy(mail.NoTLS))
	default:
		return nil, errutil.Explain(nil, "mail: unknown tls mode %q (want starttls|tls|none)", c.TLS.Mode)
	}
	if c.TLS.InsecureSkipVerify {
		opts = append(opts, mail.WithTLSConfig(&tls.Config{
			InsecureSkipVerify: true,
			ServerName:         c.Host,
		}))
	}

	client, err := mail.NewClient(c.Host, opts...)
	if err != nil {
		return nil, errutil.Explain(err, "mail: failed to create client for %s:%d", c.Host, c.Port)
	}

	// Ping (opt-in): dial the server once and close it so a bad host, port,
	// auth, or TLS setting is caught at startup instead of on the first send.
	if c.Ping {
		pctx, cancel := context.WithTimeout(ctx.Context, c.Timeout)
		defer cancel()
		if err := client.DialWithContext(pctx); err != nil {
			return nil, errutil.Explain(err, "mail: startup dial to %s:%d failed", c.Host, c.Port)
		}
		if err := client.Close(); err != nil {
			return nil, errutil.Explain(err, "mail: closing startup probe connection failed")
		}
	}

	log.Infof(ctx.Context, log.TagAppDef, "mailer created host=%s port=%d", c.Host, c.Port)
	m := &Mailer{
		client: client,
		from:   c.From,
		// The service label prefers the instance name (the config entry's key —
		// the identity the app injected the mailer by), falling back to the host
		// when the entry is unnamed.
		serviceLabel: resilience.ServiceLabel("mail", name, c.Host),
	}
	// Governance is applied HERE, while the mailer is built, so a *Mailer cannot
	// exist half-assembled: there is no applyGovernance step and nothing runs
	// afterwards. The executor comes from the container's [cloud.ClientParams],
	// the single place the resilience + fault composition lives; a hand-built
	// mailer passes the zero bundle, whose executor degrades to
	// resilience.Unmanaged — observed, with a one-time warning that no protection
	// applies — rather than silently running bare.
	params := cloud.ClientParams{Resilience: center.Resilience(), Fault: center.Fault()}
	m.exec = params.ExecutorFor("mail", m.serviceLabel)
	return m, nil
}

// Destroy releases the resilience executor. It is the gs destroy method. The
// SMTP client holds no live resource — each Send dials and closes its own
// connection — so there is nothing else to release.
func (m *Mailer) Destroy() error {
	if m.exec != nil {
		return m.exec.Close()
	}
	return nil
}

// parseAuthType maps the config string onto a go-mail SMTP auth mechanism.
func parseAuthType(s string) (mail.SMTPAuthType, error) {
	switch strings.ToLower(s) {
	case "", "auto":
		return mail.SMTPAuthAutoDiscover, nil
	case "plain":
		return mail.SMTPAuthPlain, nil
	case "login":
		return mail.SMTPAuthLogin, nil
	case "cram-md5", "crammd5":
		return mail.SMTPAuthCramMD5, nil
	default:
		return "", errutil.Explain(nil, "mail: unknown auth-type %q (want auto|plain|login|cram-md5)", s)
	}
}

// Send delivers one or more messages. It opens a single connection, sends all of
// them, and closes it. An error is returned if any message is invalid or if the
// connection or delivery fails.
//
// Each call is guarded and observed. Send declares the batch's semantic identity
// (see observe.go) on the context and routes through the mailer's resilience
// executor via [resilience.Run]; the span, the duration metrics and the access
// log are emitted by the resilience layer, the one point on the chain that sees
// the whole call. Declaring the identity is this layer's whole job — it emits
// nothing itself.
func (m *Mailer) Send(ctx context.Context, msgs ...*Message) error {
	if len(msgs) == 0 {
		return nil
	}
	ctx = observability.WithOperation(ctx, operation(msgs))
	_, err := resilience.Run(ctx, m.exec, func(attemptCtx context.Context) (struct{}, error) {
		return struct{}{}, m.deliver(attemptCtx, msgs)
	})
	return err
}

// deliver is the unobserved send: build every message, then hand the batch to
// the client in one connection.
func (m *Mailer) deliver(ctx context.Context, msgs []*Message) error {
	built := make([]*mail.Msg, 0, len(msgs))
	for _, msg := range msgs {
		mm, err := m.build(msg)
		if err != nil {
			return err
		}
		built = append(built, mm)
	}
	if err := m.client.DialAndSendWithContext(ctx, built...); err != nil {
		return errutil.Explain(err, "mail: send failed")
	}
	return nil
}

// build converts a Message into a go-mail Msg, validating the sender and
// recipients and assembling the body and attachments.
func (m *Mailer) build(msg *Message) (*mail.Msg, error) {
	from := msg.From
	if from == "" {
		from = m.from
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
