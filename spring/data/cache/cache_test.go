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

package cache_test

import (
	"errors"
	"strings"
	"testing"

	"go-spring.org/spring/data/cache"
	"go-spring.org/spring/gs"
	"go-spring.org/stdlib/testing/assert"
)

// noopDriver is a Driver that builds no beans, used to exercise the registry
// without pulling in a real backend.
var noopDriver cache.Driver = func(string) gs.ModuleFunc { return nil }

func TestJSONCodecRoundTrip(t *testing.T) {
	var c cache.JSONCodec
	in := map[string]any{"a": float64(1), "b": "x"}

	b, err := c.Marshal(in)
	assert.That(t, err).Nil()

	var out map[string]any
	assert.That(t, c.Unmarshal(b, &out)).Nil()
	assert.That(t, out).Equal(in)
}

func TestResolveCodec(t *testing.T) {
	// No override, or an explicit nil, both fall back to the default JSONCodec.
	assert.That(t, cache.ResolveCodec(nil)).Equal(cache.JSONCodec{})
	assert.That(t, cache.ResolveCodec([]cache.Codec{nil})).Equal(cache.JSONCodec{})

	// A provided codec wins.
	custom := &fakeCodec{}
	assert.That(t, cache.ResolveCodec([]cache.Codec{custom})).Equal(custom)
}

func TestErrMiss(t *testing.T) {
	// ErrMiss is a sentinel a caller distinguishes from a real backend error.
	assert.That(t, errors.Is(cache.ErrMiss, cache.ErrMiss)).True()
	assert.That(t, errors.Is(errors.New("boom"), cache.ErrMiss)).False()
}

func TestRegisterDriverPanics(t *testing.T) {
	assert.Panic(t, func() { cache.RegisterDriver("", noopDriver) }, "empty name")
	assert.Panic(t, func() { cache.RegisterDriver("test-nil-driver", nil) }, "nil driver")
}

func TestRegisterAndGetDriver(t *testing.T) {
	cache.RegisterDriver("test-noop-driver", noopDriver)

	_, err := cache.GetDriver("test-noop-driver")
	assert.That(t, err).Nil()

	// Re-registering the same name is a wiring bug; fail loudly at init.
	assert.Panic(t, func() {
		cache.RegisterDriver("test-noop-driver", noopDriver)
	}, "already registered")
}

func TestGetDriverNotFound(t *testing.T) {
	_, err := cache.GetDriver("test-does-not-exist")
	assert.That(t, err).NotNil()
	// The error lists what IS registered so a typo or a missing starter import
	// is obvious.
	assert.That(t, strings.Contains(err.Error(), "no driver registered")).True()
}

// fakeCodec is a no-op Codec used only to prove ResolveCodec returns the
// caller's codec by identity rather than the default.
type fakeCodec struct{}

func (fakeCodec) Marshal(v any) ([]byte, error)  { return nil, nil }
func (fakeCodec) Unmarshal(b []byte, v any) error { return nil }
