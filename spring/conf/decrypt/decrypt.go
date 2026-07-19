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

// Package decrypt provides property-level decryption for the conf binding
// pipeline, the Go-Spring equivalent of Jasypt's ENC(...) and Spring Cloud
// Config's {cipher}... markers.
//
// A property whose resolved value is wrapped in one of the recognized markers
//
//	password=ENC(<ciphertext>)
//	password={cipher}<ciphertext>
//
// is decrypted right before it is bound, so application code and downstream
// starters only ever see the plaintext. Plain (unwrapped) values pass through
// untouched, so enabling the feature has no effect on configuration that does
// not use a marker.
//
// The decryptor is pluggable through a driver registry that mirrors the
// conf provider/converter registries (panic on empty/nil/duplicate). A
// symmetric AES-GCM driver ("aes") ships built in; a company can register its
// own driver — an asymmetric scheme or a cloud KMS client — and select it with
// the GS_CONFIG_DECRYPT_DRIVER environment variable without forking conf.
//
// The decryption key itself never lives in a configuration file: the built-in
// driver reads it from an environment variable or a mounted file (see aes.go).
// A value that carries a marker but cannot be decrypted fails startup with a
// clear errutil error rather than degrading to a half-working default.
package decrypt

import (
	"os"
	"strings"
	"sync"

	"go-spring.org/stdlib/errutil"
)

// Decryptor turns a ciphertext (the inner text of an ENC(...) / {cipher}...
// marker) into its plaintext form.
type Decryptor interface {
	Decrypt(cipherText string) (plainText string, err error)
}

// Factory builds a Decryptor. It is invoked lazily, the first time a marked
// property is encountered, and reads its key material from the environment or a
// mounted file at that point. Returning an error makes the marked property fail
// to bind (fail-fast).
type Factory func() (Decryptor, error)

// DefaultDriver is the driver used when GS_CONFIG_DECRYPT_DRIVER is unset.
const DefaultDriver = "aes"

// EnvDriver selects the active decryptor driver by name. It exists so a
// company driver can be chosen without a code change; when unset the built-in
// AES-GCM driver is used.
const EnvDriver = "GS_CONFIG_DECRYPT_DRIVER"

var drivers = map[string]Factory{}

// RegisterDriver registers a decryptor Factory under name. It follows the same
// panic-on-empty/nil/duplicate convention as the other conf registries and must
// be called from an init function only.
func RegisterDriver(name string, f Factory) {
	if name == "" {
		panic("decrypt driver name cannot be empty")
	}
	if f == nil {
		panic("decrypt driver " + name + " cannot be nil")
	}
	if _, ok := drivers[name]; ok {
		panic("decrypt driver " + name + " already exists")
	}
	drivers[name] = f
}

// Markers recognized on a resolved property value. Both the Jasypt-style
// ENC(...) wrapper and the Spring Cloud Config {cipher} prefix are accepted.
const (
	encPrefix    = "ENC("
	encSuffix    = ")"
	cipherPrefix = "{cipher}"
)

// unwrap reports whether value carries a decryption marker and, if so, returns
// the inner ciphertext. A value without a marker returns ("", false).
func unwrap(value string) (cipherText string, marked bool) {
	if strings.HasPrefix(value, cipherPrefix) {
		return value[len(cipherPrefix):], true
	}
	if strings.HasPrefix(value, encPrefix) && strings.HasSuffix(value, encSuffix) {
		return value[len(encPrefix) : len(value)-len(encSuffix)], true
	}
	return "", false
}

// active caches the resolved decryptor (and any build error) so the key is read
// and the driver constructed only once per process.
var (
	activeOnce sync.Once
	activeDec  Decryptor
	activeErr  error
)

// activeDecryptor resolves and caches the decryptor selected by the
// GS_CONFIG_DECRYPT_DRIVER environment variable.
func activeDecryptor() (Decryptor, error) {
	activeOnce.Do(func() {
		name := os.Getenv(EnvDriver)
		if name == "" {
			name = DefaultDriver
		}
		f, ok := drivers[name]
		if !ok {
			activeErr = errutil.Explain(nil, "unknown decrypt driver %q (set %s)", name, EnvDriver)
			return
		}
		activeDec, activeErr = f()
	})
	return activeDec, activeErr
}

// Decode returns the plaintext for a resolved property value. Values without a
// decryption marker are returned unchanged. A marked value is decrypted with
// the active driver; a build or decrypt failure is surfaced so the caller can
// fail startup.
func Decode(value string) (string, error) {
	cipherText, marked := unwrap(value)
	if !marked {
		return value, nil
	}
	dec, err := activeDecryptor()
	if err != nil {
		return "", err
	}
	plain, err := dec.Decrypt(cipherText)
	if err != nil {
		return "", errutil.Explain(err, "decrypt property value failed")
	}
	return plain, nil
}
