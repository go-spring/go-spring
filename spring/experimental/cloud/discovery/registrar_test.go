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
	"context"
	"testing"

	"go-spring.org/stdlib/testing/assert"
)

// staticRegistrar records the last Register/Deregister call for assertions.
type staticRegistrar struct {
	registered   []Registration
	deregistered []Registration
}

func (r *staticRegistrar) Register(_ context.Context, reg Registration) error {
	r.registered = append(r.registered, reg)
	return nil
}

func (r *staticRegistrar) Deregister(_ context.Context, reg Registration) error {
	r.deregistered = append(r.deregistered, reg)
	return nil
}

func TestRegisterRegistrar(t *testing.T) {
	r := &staticRegistrar{}
	RegisterRegistrar("test-registrar", r)

	got, ok := GetRegistrar("test-registrar")
	assert.That(t, ok).True()
	assert.That(t, got).Equal(Registrar(r))

	_, ok = GetRegistrar("missing")
	assert.That(t, ok).False()
}

func TestRegisterRegistrar_DuplicatePanics(t *testing.T) {
	RegisterRegistrar("test-registrar-dup", &staticRegistrar{})
	assert.Panic(t, func() {
		RegisterRegistrar("test-registrar-dup", &staticRegistrar{})
	}, "already registered")
}

func TestRegisterRegistrar_EmptyOrNilPanics(t *testing.T) {
	assert.Panic(t, func() { RegisterRegistrar("", &staticRegistrar{}) }, "empty name")
	assert.Panic(t, func() { RegisterRegistrar("test-registrar-nil", nil) }, "nil registrar")
}

func TestMustGetRegistrar(t *testing.T) {
	r := &staticRegistrar{}
	RegisterRegistrar("test-registrar-mustget", r)

	got, err := MustGetRegistrar("test-registrar-mustget")
	assert.Error(t, err).Nil()
	assert.That(t, got).Equal(Registrar(r))

	_, err = MustGetRegistrar("test-registrar-mustget-missing")
	assert.Error(t, err).Matches("no registrar registered")
}
