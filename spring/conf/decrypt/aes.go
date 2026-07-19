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
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"os"
	"strings"

	"go-spring.org/stdlib/errutil"
)

// Environment variables the built-in AES driver reads its key from. The key
// never lives in a configuration file: it is supplied out of band through the
// process environment or a mounted file (a Kubernetes Secret, a Vault Agent
// sink, ...). EnvKey wins over EnvKeyFile when both are set.
const (
	// EnvKey holds the base64-encoded AES key directly.
	EnvKey = "GS_CONFIG_DECRYPT_KEY"
	// EnvKeyFile holds a path to a file containing the base64-encoded key.
	EnvKeyFile = "GS_CONFIG_DECRYPT_KEY_FILE"
)

func init() {
	RegisterDriver(DefaultDriver, newAESDecryptor)
}

// aesDecryptor decrypts values produced by Encrypt: AES-GCM with a 12-byte
// random nonce prepended to the ciphertext, the whole thing base64-encoded.
type aesDecryptor struct {
	aead cipher.AEAD
}

// newAESDecryptor loads the key from the environment and builds an AES-GCM
// decryptor. A missing or malformed key is a startup error (fail-fast) rather
// than a silent no-op.
func newAESDecryptor() (Decryptor, error) {
	key, err := loadKey()
	if err != nil {
		return nil, err
	}
	aead, err := newGCM(key)
	if err != nil {
		return nil, err
	}
	return &aesDecryptor{aead: aead}, nil
}

// loadKey reads the base64-encoded AES key from EnvKey or EnvKeyFile and
// decodes it. The decoded key length must be 16, 24, or 32 bytes (AES-128/192/
// 256).
func loadKey() ([]byte, error) {
	var encoded string
	if v := os.Getenv(EnvKey); v != "" {
		encoded = v
	} else if path := os.Getenv(EnvKeyFile); path != "" {
		b, err := os.ReadFile(path)
		if err != nil {
			return nil, errutil.Explain(err, "read decrypt key file %q failed", path)
		}
		encoded = string(b)
	} else {
		return nil, errutil.Explain(nil, "no AES decrypt key configured (set %s or %s)", EnvKey, EnvKeyFile)
	}

	key, err := base64.StdEncoding.DecodeString(strings.TrimSpace(encoded))
	if err != nil {
		return nil, errutil.Explain(err, "decrypt key is not valid base64")
	}
	switch len(key) {
	case 16, 24, 32:
		return key, nil
	default:
		return nil, errutil.Explain(nil, "decrypt key must decode to 16, 24, or 32 bytes, got %d", len(key))
	}
}

// newGCM builds an AES-GCM AEAD from the raw key.
func newGCM(key []byte) (cipher.AEAD, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, errutil.Explain(err, "create AES cipher failed")
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return nil, errutil.Explain(err, "create AES-GCM failed")
	}
	return aead, nil
}

// Decrypt reverses Encrypt: base64-decode, split the leading nonce, and open
// the AES-GCM sealed box.
func (d *aesDecryptor) Decrypt(cipherText string) (string, error) {
	raw, err := base64.StdEncoding.DecodeString(strings.TrimSpace(cipherText))
	if err != nil {
		return "", errutil.Explain(err, "ciphertext is not valid base64")
	}
	ns := d.aead.NonceSize()
	if len(raw) < ns {
		return "", errutil.Explain(nil, "ciphertext too short")
	}
	nonce, sealed := raw[:ns], raw[ns:]
	plain, err := d.aead.Open(nil, nonce, sealed, nil)
	if err != nil {
		return "", errutil.Explain(err, "AES-GCM open failed (wrong key or corrupted ciphertext)")
	}
	return string(plain), nil
}

// Encrypt is the counterpart used by tooling and tests to produce ENC(...)
// values. It reads the same key from the environment, seals plainText under
// AES-GCM with a fresh random nonce, and returns the base64 of nonce||sealed —
// the exact form Decrypt expects inside an ENC(...) / {cipher} marker.
func Encrypt(plainText string) (string, error) {
	key, err := loadKey()
	if err != nil {
		return "", err
	}
	aead, err := newGCM(key)
	if err != nil {
		return "", err
	}
	nonce := make([]byte, aead.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return "", errutil.Explain(err, "generate nonce failed")
	}
	sealed := aead.Seal(nil, nonce, []byte(plainText), nil)
	return base64.StdEncoding.EncodeToString(append(nonce, sealed...)), nil
}
