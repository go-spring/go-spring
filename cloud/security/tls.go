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

package security

import (
	"crypto/tls"
	"crypto/x509"
	"os"

	"go-spring.org/stdlib/errutil"
)

// tlsErrPrefix heads every error this file returns: the builders serve many
// components, so the prefix names the protocol rather than the caller.
const tlsErrPrefix = "tls:"

// TLSConfig is the nested, off-by-default TLS block shared by every starter.
// It is the union of the fields the starters were each declaring on their own:
// a client/server key pair (CertFile/KeyFile), a CA bundle to verify the peer
// (CAFile), the expected peer name (ServerName), and an escape hatch for local
// testing (InsecureSkipVerify).
//
// Embed it under a `tls` key so the bound properties read as e.g.
// `${spring.redis.tls.enabled}`, `${spring.redis.tls.cert-file}`, ...
type TLSConfig struct {
	// Enabled turns TLS on. It defaults to false so a starter never negotiates
	// TLS unless the operator asks for it.
	Enabled bool `value:"${enabled:=false}"`

	// CertFile and KeyFile are the PEM key pair this side presents. Leave both
	// empty when no client/peer certificate is required.
	CertFile string `value:"${cert-file:=}"`
	KeyFile  string `value:"${key-file:=}"`

	// CAFile is a PEM bundle of root CAs used to verify the peer certificate.
	// When empty the host's default root set is used.
	CAFile string `value:"${ca-file:=}"`

	// ServerName overrides the name checked against the peer certificate, useful
	// when dialing by IP or through a service-discovery label.
	ServerName string `value:"${server-name:=}"`

	// InsecureSkipVerify disables peer certificate verification. Intended for
	// local testing only - never enable it in production.
	InsecureSkipVerify bool `value:"${insecure-skip-verify:=false}"`
}

// BuildClient turns the config into a client-side *tls.Config, or (nil, nil)
// when TLS is disabled so a caller can pass the result straight through to a
// library that treats a nil *tls.Config as "no TLS". It loads the key pair
// and CA bundle from disk when provided.
//
// Errors are wrapped with a generic "tls:" prefix; a starter that wants a
// component-specific prefix should wrap the returned error with
// errutil.Explain(err, "redis: ...").
func (c TLSConfig) BuildClient() (_ *tls.Config, err error) {
	if !c.Enabled {
		return nil, nil
	}
	cfg := &tls.Config{
		ServerName:         c.ServerName,
		InsecureSkipVerify: c.InsecureSkipVerify,
	}
	cfg.Certificates, err = c.loadCertificates()
	if err != nil {
		return nil, err
	}
	if c.CAFile != "" {
		cfg.RootCAs, err = c.loadCAPool()
		if err != nil {
			return nil, err
		}
	}
	return cfg, nil
}

// BuildServer turns the config into a server-side *tls.Config, or (nil, nil)
// when TLS is disabled. Server semantics differ from [TLSConfig.BuildClient]
// in three ways:
//
//   - CAFile is the bundle of CAs trusted to sign CLIENT certificates: it sets
//     ClientCAs and turns on RequireAndVerifyClientCert, i.e. requesting a CA
//     file enables mutual TLS. Leave it empty for one-way TLS.
//   - ServerName and InsecureSkipVerify are client-side knobs; on the server
//     they describe verifying the peer WE dial, so both are ignored here.
//   - A key pair is required: a TLS server without a certificate cannot
//     complete a handshake, so an enabled server config with no cert-file/
//     key-file is rejected here instead of failing at connection time.
//
// Errors are wrapped with the same generic "tls:" prefix as [TLSConfig.BuildClient].
func (c TLSConfig) BuildServer() (_ *tls.Config, err error) {
	if !c.Enabled {
		return nil, nil
	}
	if c.CertFile == "" && c.KeyFile == "" {
		return nil, errutil.Explain(nil, "%s server TLS requires cert-file and key-file", tlsErrPrefix)
	}
	cfg := &tls.Config{}
	cfg.Certificates, err = c.loadCertificates()
	if err != nil {
		return nil, err
	}
	if c.CAFile != "" {
		cfg.ClientCAs, err = c.loadCAPool()
		if err != nil {
			return nil, err
		}
		cfg.ClientAuth = tls.RequireAndVerifyClientCert
	}
	return cfg, nil
}

// loadCertificates loads the configured key pair. No pair configured is (nil,
// nil) — legal for a client. A half-configured pair is rejected here rather
// than passed to tls.LoadX509KeyPair, which would fail with an empty path in
// the message that hides the real mistake.
func (c TLSConfig) loadCertificates() ([]tls.Certificate, error) {
	switch {
	case c.CertFile == "" && c.KeyFile == "":
		return nil, nil
	case c.CertFile == "":
		return nil, errutil.Explain(nil, "%s key-file is set but cert-file is empty", tlsErrPrefix)
	case c.KeyFile == "":
		return nil, errutil.Explain(nil, "%s cert-file is set but key-file is empty", tlsErrPrefix)
	}
	cert, err := tls.LoadX509KeyPair(c.CertFile, c.KeyFile)
	if err != nil {
		return nil, errutil.Explain(err, "%s failed to load key pair", tlsErrPrefix)
	}
	return []tls.Certificate{cert}, nil
}

// loadCAPool reads the CA bundle from disk into an x509 pool.
func (c TLSConfig) loadCAPool() (*x509.CertPool, error) {
	pem, err := os.ReadFile(c.CAFile)
	if err != nil {
		return nil, errutil.Explain(err, "%s failed to read CA file", tlsErrPrefix)
	}
	pool := x509.NewCertPool()
	if !pool.AppendCertsFromPEM(pem) {
		return nil, errutil.Explain(nil, "%s no certificates found in CA file %s", tlsErrPrefix, c.CAFile)
	}
	return pool, nil
}
