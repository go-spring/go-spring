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

package conf_test

import (
	"testing"

	"go-spring.org/spring/conf"
	"go-spring.org/spring/conf/decrypt"
	"go-spring.org/stdlib/flatten"
	"go-spring.org/stdlib/testing/assert"
)

// key128 is the base64 of the 16-byte AES key "1234567890123456".
const key128 = "MTIzNDU2Nzg5MDEyMzQ1Ng=="

// TestBindDecryptsMarkedValue verifies an ENC(...) property is decrypted before
// it is bound to a struct field.
func TestBindDecryptsMarkedValue(t *testing.T) {
	t.Setenv(decrypt.EnvKey, key128)

	enc, err := decrypt.Encrypt("s3cr3t")
	assert.Error(t, err).Nil()

	p := flatten.NewPropertiesStorage(flatten.MapProperties(map[string]any{
		"db.user":     "admin",
		"db.password": "ENC(" + enc + ")",
	}))

	var cfg struct {
		User     string `value:"${db.user}"`
		Password string `value:"${db.password}"`
	}
	err = conf.Bind(p, &cfg)
	assert.Error(t, err).Nil()
	assert.That(t, cfg.User).Equal("admin")
	assert.That(t, cfg.Password).Equal("s3cr3t")
}

// TestBindDecryptFailsFast verifies a value that cannot be decrypted makes the
// bind fail with a clear error rather than binding garbage. A corrupt
// ciphertext is used so the outcome does not depend on the process-wide key
// cache that the happy-path test primes.
func TestBindDecryptFailsFast(t *testing.T) {
	t.Setenv(decrypt.EnvKey, key128)

	p := flatten.NewPropertiesStorage(flatten.MapProperties(map[string]any{
		"db.password": "ENC(not!valid!base64!)",
	}))
	var cfg struct {
		Password string `value:"${db.password}"`
	}
	err := conf.Bind(p, &cfg)
	assert.Error(t, err).Matches("failed to decrypt value at path")
}
