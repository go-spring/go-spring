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

package StarterAsynq

import (
	"context"
	"testing"

	"github.com/hibiken/asynq"
	observe "go-spring.org/cloud/observe"
	"go-spring.org/spring/gs"
	"go-spring.org/stdlib/testing/assert"
)

// TestDefaultDriverPlain builds the plain RedisConnOpt: addr/user/pass/db map
// through unchanged, and TLS is not enabled.
func TestDefaultDriverPlain(t *testing.T) {
	d := DefaultDriver{}
	opt, err := d.RedisConnOpt(context.Background(), Config{
		Addr: "127.0.0.1:6379", Username: "u", Password: "p", DB: 2,
	})
	assert.Error(t, err).Nil()
	ro := opt.(asynq.RedisClientOpt)
	assert.That(t, ro.Addr).Equal("127.0.0.1:6379")
	assert.That(t, ro.Username).Equal("u")
	assert.That(t, ro.Password).Equal("p")
	assert.That(t, ro.DB).Equal(2)
	assert.That(t, ro.TLSConfig == nil).True()
}

// TestConfigDefaults pins the server opt-in: the worker role is off unless
// explicitly enabled.
func TestConfigDefaults(t *testing.T) {
	var c Config
	assert.That(t, c.Server.Enabled).False()
	assert.That(t, c.Concurrency).Equal(0) // zero value; asynq default is 10 at runtime
}

// TestResolveObservability pins the precedence between the two observability
// config surfaces: instance-prefixed spring.asynq.<name>.observability.*
// (bound into Config.Observability by BindEach) overrides the top-level
// observability.* keys field-injected into the wrapper. Binding fills the
// defaults (brief/512/no skips) even when no instance key is present, so only
// non-default instance values count as "set".
func TestResolveObservability(t *testing.T) {
	// Instance unset (binding defaults) -> top-level field wins untouched.
	w := &Client{Observability: observe.ObserveConfig{
		Level: "detailed", MaxArgBytes: 1024, SkipOps: []string{"enqueue/x"},
	}}
	w.cfg.Observability = observe.ObserveConfig{Level: "brief", MaxArgBytes: 512}
	got := w.resolveObservability()
	assert.That(t, got.Level).Equal("detailed")
	assert.That(t, got.MaxArgBytes).Equal(1024)
	assert.That(t, got.SkipOps).Equal([]string{"enqueue/x"})

	// Instance set -> overrides top-level per field.
	w.cfg.Observability = observe.ObserveConfig{
		Level: "off", MaxArgBytes: 2048, SkipOps: []string{"enqueue/y"},
	}
	got = w.resolveObservability()
	assert.That(t, got.Level).Equal("off")
	assert.That(t, got.MaxArgBytes).Equal(2048)
	assert.That(t, got.SkipOps).Equal([]string{"enqueue/y"})

	// Partial instance override: only the field set to a non-default value
	// changes; the rest keeps the top-level value.
	w.cfg.Observability = observe.ObserveConfig{Level: "off", MaxArgBytes: 512}
	got = w.resolveObservability()
	assert.That(t, got.Level).Equal("off")
	assert.That(t, got.MaxArgBytes).Equal(1024)
}

// fakeDriver is a Driver stub recording that it was selected.
type fakeDriver struct{ called bool }

func (f *fakeDriver) RedisConnOpt(ctx context.Context, c Config) (asynq.RedisConnOpt, error) {
	f.called = true
	return asynq.RedisClientOpt{Addr: "fake:" + c.Addr}, nil
}

// TestNewClientDriverLookup pins the driver registry wiring: the configured
// driver key selects the registered driver (default DefaultDriver when unset
// or empty), and an unregistered name is a clear startup error instead of a
// silent fallback.
func TestNewClientDriverLookup(t *testing.T) {
	fd := &fakeDriver{}
	RegisterDriver("test-fake-driver", fd)
	t.Cleanup(func() { delete(driverRegistry, "test-fake-driver") })

	cp := &gs.ContextProvider{Context: context.Background()}

	// Default: unset and empty both resolve to DefaultDriver.
	c, err := newClient(cp, Config{Addr: "127.0.0.1:6379"})
	assert.Error(t, err).Nil()
	_ = c.Close()
	assert.That(t, fd.called).False()

	c, err = newClient(cp, Config{Addr: "127.0.0.1:6379", Driver: ""})
	assert.Error(t, err).Nil()
	_ = c.Close()
	assert.That(t, fd.called).False()

	// Configured name -> the registered custom driver is actually used.
	c, err = newClient(cp, Config{Addr: "127.0.0.1:6379", Driver: "test-fake-driver"})
	assert.Error(t, err).Nil()
	_ = c.Close()
	assert.That(t, fd.called).True()

	// Unknown name -> clear error, no fallback.
	_, err = newClient(cp, Config{Addr: "127.0.0.1:6379", Driver: "no-such-driver"})
	assert.That(t, err != nil).True()
}
