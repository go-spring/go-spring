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

package starter

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTLSConfig_Build_Disabled(t *testing.T) {
	cfg, err := TLSConfig{Enabled: false, CertFile: "nope"}.Build()
	assert.NoError(t, err)
	assert.Nil(t, cfg, "disabled TLS must return a nil *tls.Config")
}

func TestTLSConfig_Build_EnabledNoFiles(t *testing.T) {
	cfg, err := TLSConfig{Enabled: true, ServerName: "peer", InsecureSkipVerify: true}.Build()
	require.NoError(t, err)
	require.NotNil(t, cfg)
	assert.Equal(t, "peer", cfg.ServerName)
	assert.True(t, cfg.InsecureSkipVerify)
	assert.Empty(t, cfg.Certificates)
	assert.Nil(t, cfg.RootCAs)
}

func TestTLSConfig_Build_MissingCert(t *testing.T) {
	_, err := TLSConfig{Enabled: true, CertFile: "/does/not/exist.pem", KeyFile: "/nope.key"}.Build()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "tls: failed to load key pair")
}

func TestTLSConfig_Build_MissingCAFile(t *testing.T) {
	_, err := TLSConfig{Enabled: true, CAFile: "/does/not/exist-ca.pem"}.Build()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "tls: failed to read CA file")
}

func TestTLSConfig_Build_BadCAContent(t *testing.T) {
	dir := t.TempDir()
	caPath := filepath.Join(dir, "ca.pem")
	require.NoError(t, os.WriteFile(caPath, []byte("not a certificate"), 0o600))

	_, err := TLSConfig{Enabled: true, CAFile: caPath}.Build()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no certificates found in CA file")
}

func TestTLSConfig_Build_ValidKeyPairAndCA(t *testing.T) {
	dir := t.TempDir()
	certPath, keyPath := writeSelfSignedPair(t, dir)

	cfg, err := TLSConfig{Enabled: true, CertFile: certPath, KeyFile: keyPath, CAFile: certPath}.Build()
	require.NoError(t, err)
	require.NotNil(t, cfg)
	assert.Len(t, cfg.Certificates, 1)
	assert.NotNil(t, cfg.RootCAs)
}

// writeSelfSignedPair writes a self-signed cert/key pair and returns their
// paths. The cert doubles as a valid CA bundle for the CAFile test.
func writeSelfSignedPair(t *testing.T, dir string) (certPath, keyPath string) {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	tmpl := &x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject:      pkix.Name{CommonName: "test"},
		NotBefore:    time.Now().Add(-time.Hour),
		NotAfter:     time.Now().Add(time.Hour),
		IsCA:         true,
	}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &key.PublicKey, key)
	require.NoError(t, err)

	certPath = filepath.Join(dir, "cert.pem")
	keyPath = filepath.Join(dir, "key.pem")
	certPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der})
	keyPEM := pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(key)})
	require.NoError(t, os.WriteFile(certPath, certPEM, 0o600))
	require.NoError(t, os.WriteFile(keyPath, keyPEM, 0o600))
	return certPath, keyPath
}
