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
	"crypto/tls"
	"strings"

	"github.com/wneessen/go-mail"
	"go-spring.org/cloud"
	"go-spring.org/cloud/governance"
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
			).Name(name).Destroy((*Mailer).Close).Caller(1)
			return nil
		})
	})
}

// newMailer builds a Mailer from config. It fails fast on a missing host or an
// unknown auth/TLS mode, and (when Ping is enabled) probes the server once at
// startup so a misconfiguration surfaces at boot rather than on the first send.
//
// center is the governance center the container injects — the family's sole
// injection point; the ctor reads the resilience and fault authorities from it
// and bundles them into the [cloud.ClientParams] the chain's governance layer
// builds its executor from, so the mailer is assembled complete in one step.
// The zero bundle degrades to an observed-only, loudly-unmanaged executor.
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
	// The chain owns everything, one statement per layer: identity over
	// governance over the raw adapter. Governance is applied HERE, while the
	// mailer is built, so a *Mailer cannot exist half-assembled; a hand-built
	// mailer passes the zero bundle, whose executor degrades to
	// resilience.Unmanaged — observed, with a one-time warning that no protection
	// applies — rather than silently running bare.
	params := cloud.ClientParams{Resilience: center.Resilience(), Fault: center.Fault()}
	raw := NewRawMailer(client, c.From)
	guard := NewGuardMailer(raw, name, c.Host, params)
	return &Mailer{Client: client, from: c.From, InnerMailer: NewObsMailer(guard)}, nil
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
