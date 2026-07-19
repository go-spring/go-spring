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
	"time"
)

// Config defines an SMTP mailer's connection and authentication settings.
type Config struct {
	// Host is the SMTP server hostname, e.g., "smtp.example.com". It is
	// required; there is no localhost fallback (fail-fast at startup).
	Host string `value:"${host:=}"`

	// Port is the SMTP server port. 587 (submission + STARTTLS) is the default;
	// use 465 with tls.mode=tls for implicit TLS, or 25/1025 for plaintext.
	Port int `value:"${port:=587}"`

	// Username is the SMTP auth username. When empty, no authentication is
	// performed and the mailer connects anonymously.
	Username string `value:"${username:=}"`

	// Password is the SMTP auth password, used together with Username.
	Password string `value:"${password:=}"`

	// AuthType selects the SMTP authentication mechanism. It is only consulted
	// when Username is set. One of: "auto" (negotiate the strongest mechanism
	// the server offers), "plain", "login", "cram-md5". Default is "auto".
	AuthType string `value:"${auth-type:=auto}"`

	// From is the default sender address used when a Message does not set its
	// own From. It may be empty, in which case every Message must set From.
	From string `value:"${from:=}"`

	// Timeout bounds both the startup connection probe and each send's dial.
	// Default is "10s".
	Timeout time.Duration `value:"${timeout:=10s}"`

	// TLS configures transport security for the connection.
	TLS TLSConfig `value:"${tls}"`
}

// TLSConfig configures transport security for the SMTP connection. Mode selects
// how encryption is negotiated; the remaining fields refine certificate
// verification.
type TLSConfig struct {
	// Mode selects the transport security policy:
	//   - "starttls" (default): connect in plaintext then upgrade via STARTTLS,
	//     required (the connection fails if the server does not offer it).
	//   - "tls": implicit TLS from the first byte (SMTPS, typically port 465).
	//   - "none": no encryption; intended for local test servers only.
	Mode string `value:"${mode:=starttls}"`

	// InsecureSkipVerify disables server certificate verification. It is
	// intended for testing only and must not be used in production.
	InsecureSkipVerify bool `value:"${insecure-skip-verify:=false}"`
}
