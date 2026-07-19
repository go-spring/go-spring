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

package decrypt

import (
	"encoding/base64"
	"os"
	"sync"
	"testing"

	"go-spring.org/stdlib/testing/assert"
)

// resetActive clears the cached active decryptor so a test can change the
// environment and observe a fresh lookup. The package caches the decryptor via
// sync.Once for production use; tests reach in to reset it.
func resetActive() {
	activeOnce = sync.Once{}
	activeDec = nil
	activeErr = nil
}

// key128 is a 16-byte AES key, base64-encoded, used across the tests.
const key128 = "MTIzNDU2Nzg5MDEyMzQ1Ng==" // "1234567890123456"

func TestUnwrap(t *testing.T) {
	cases := []struct {
		in     string
		cipher string
		marked bool
	}{
		{"plain", "", false},
		{"ENC(abc)", "abc", true},
		{"{cipher}abc", "abc", true},
		{"ENC(", "", false},    // missing closing paren
		{"ENC()", "", true},    // empty ciphertext still marked
		{"{cipher}", "", true}, // empty ciphertext still marked
		{"prefixENC(abc)", "", false},
	}
	for _, c := range cases {
		cipher, marked := unwrap(c.in)
		assert.That(t, marked).Equal(c.marked, c.in)
		assert.That(t, cipher).Equal(c.cipher, c.in)
	}
}

func TestAESRoundTrip(t *testing.T) {
	t.Setenv(EnvKey, key128)
	resetActive()

	plain := "s3cr3t-password"
	enc, err := Encrypt(plain)
	assert.Error(t, err).Nil()

	got, err := Decode("ENC(" + enc + ")")
	assert.Error(t, err).Nil()
	assert.That(t, got).Equal(plain)

	got, err = Decode("{cipher}" + enc)
	assert.Error(t, err).Nil()
	assert.That(t, got).Equal(plain)
}

func TestDecodePlainPassthrough(t *testing.T) {
	// No marker: value returns unchanged and no decryptor is built, so an
	// absent key must not matter.
	t.Setenv(EnvKey, "")
	resetActive()
	got, err := Decode("localhost:5432")
	assert.Error(t, err).Nil()
	assert.That(t, got).Equal("localhost:5432")
}

func TestDecodeMissingKeyFailsFast(t *testing.T) {
	t.Setenv(EnvKey, "")
	t.Setenv(EnvKeyFile, "")
	resetActive()
	_, err := Decode("ENC(whatever)")
	assert.Error(t, err).Matches("no AES decrypt key configured")
}

func TestDecodeWrongKeyFailsFast(t *testing.T) {
	// Encrypt under one key ...
	t.Setenv(EnvKey, key128)
	resetActive()
	enc, err := Encrypt("hello")
	assert.Error(t, err).Nil()

	// ... then attempt to decrypt under a different key.
	otherKey := base64.StdEncoding.EncodeToString([]byte("abcdefghijklmnop"))
	t.Setenv(EnvKey, otherKey)
	resetActive()
	_, err = Decode("ENC(" + enc + ")")
	assert.Error(t, err).Matches("AES-GCM open failed")
}

func TestDecodeBadBase64(t *testing.T) {
	t.Setenv(EnvKey, key128)
	resetActive()
	_, err := Decode("ENC(not!base64!)")
	assert.Error(t, err).Matches("ciphertext is not valid base64")
}

func TestLoadKeyFromFile(t *testing.T) {
	dir := t.TempDir()
	path := dir + "/aes.key"
	err := os.WriteFile(path, []byte(key128), 0600)
	assert.Error(t, err).Nil()

	t.Setenv(EnvKey, "")
	t.Setenv(EnvKeyFile, path)
	resetActive()

	enc, err := Encrypt("from-file")
	assert.Error(t, err).Nil()
	got, err := Decode("ENC(" + enc + ")")
	assert.Error(t, err).Nil()
	assert.That(t, got).Equal("from-file")
}

func TestBadKeyLength(t *testing.T) {
	t.Setenv(EnvKey, base64.StdEncoding.EncodeToString([]byte("too-short")))
	resetActive()
	_, err := Decode("ENC(anything)")
	assert.Error(t, err).Matches("decrypt key must decode to 16, 24, or 32 bytes")
}

func TestRegisterDriverPanics(t *testing.T) {
	assert.Panic(t, func() { RegisterDriver("", func() (Decryptor, error) { return nil, nil }) }, "cannot be empty")
	assert.Panic(t, func() { RegisterDriver("x", nil) }, "cannot be nil")
	assert.Panic(t, func() { RegisterDriver(DefaultDriver, func() (Decryptor, error) { return nil, nil }) }, "already exists")
}

func TestUnknownDriver(t *testing.T) {
	t.Setenv(EnvDriver, "nope")
	resetActive()
	_, err := Decode("ENC(x)")
	assert.Error(t, err).Matches("unknown decrypt driver")
}
