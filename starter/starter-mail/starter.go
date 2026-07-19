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
	"strings"

	"github.com/wneessen/go-mail"
	"go-spring.org/spring/gs"
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
// long-lived socket is kept between calls (hence no destroy hook is needed).
type Mailer struct {
	client *mail.Client
	from   string
}

func init() {
	// Register multiple SMTP mailers as a group. Each instance is created from
	// the configuration under "${spring.mail}", so adding a second mailer is a
	// pure-config change. There is no default singleton — select one by name
	// (e.g. autowire:"notify").
	//
	// No destroy callback: the underlying client opens a fresh connection per
	// Send and closes it when done, so there is nothing to release at shutdown.
	gs.Group("${spring.mail}", newMailer, nil)
}

// newMailer builds a Mailer from config. It fails fast on a missing host or an
// unknown auth/TLS mode, and probes the server once at startup so a
// misconfiguration surfaces at boot rather than on the first send.
func newMailer(c Config) (*Mailer, error) {
	if c.Host == "" {
		return nil, errutil.Explain(nil, "mail: host is required")
	}

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

	// Fail fast: dial the server once and close it so a bad host, port, auth, or
	// TLS setting is caught at startup instead of on the first send.
	ctx, cancel := context.WithTimeout(context.Background(), c.Timeout)
	defer cancel()
	if err := client.DialWithContext(ctx); err != nil {
		return nil, errutil.Explain(err, "mail: startup dial to %s:%d failed", c.Host, c.Port)
	}
	if err := client.Close(); err != nil {
		return nil, errutil.Explain(err, "mail: closing startup probe connection failed")
	}

	return &Mailer{client: client, from: c.From}, nil
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
func (m *Mailer) Send(ctx context.Context, msgs ...*Message) error {
	if len(msgs) == 0 {
		return nil
	}
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
		return nil, errutil.Explain(nil, "mail: no From address (set message.From or spring.mail...from)")
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
