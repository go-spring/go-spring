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

package discovery

import (
	"testing"

	"go-spring.org/stdlib/testing/assert"
)

func TestRegister(t *testing.T) {
	d := newStaticDiscovery()
	Register("test-register", d)

	got, ok := Get("test-register")
	assert.That(t, ok).True()
	assert.That(t, got).Equal(Discovery(d))

	_, ok = Get("missing")
	assert.That(t, ok).False()
}

func TestRegister_DuplicatePanics(t *testing.T) {
	Register("test-dup", newStaticDiscovery())
	assert.Panic(t, func() {
		Register("test-dup", newStaticDiscovery())
	}, "already registered")
}

func TestRegister_EmptyOrNilPanics(t *testing.T) {
	assert.Panic(t, func() { Register("", newStaticDiscovery()) }, "empty name")
	assert.Panic(t, func() { Register("test-nil", nil) }, "nil backend")
}

func TestMustGet(t *testing.T) {
	d := newStaticDiscovery()
	Register("test-mustget", d)

	got, err := MustGet("test-mustget")
	assert.Error(t, err).Nil()
	assert.That(t, got).Equal(Discovery(d))

	_, err = MustGet("test-mustget-missing")
	assert.Error(t, err).Matches("no backend registered")
}
