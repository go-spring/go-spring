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

// fakeDriver is a Driver stub recording that it was used.
type fakeDriver struct{ called bool }

func (f *fakeDriver) RedisConnOpt(ctx context.Context, c Config) (asynq.RedisConnOpt, error) {
	f.called = true
	return asynq.RedisClientOpt{Addr: "fake:" + c.Addr}, nil
}

// TestNewClientDriverFallback pins the optional-Driver-bean semantics: a nil
// Driver (no bean provided) falls back to the bundled DefaultDriver, while a
// provided Driver (a bean override) is used directly.
func TestNewClientDriverFallback(t *testing.T) {
	cp := &gs.ContextProvider{Context: context.Background()}

	// No Driver bean → the bundled DefaultDriver assembles the client.
	c, err := newClient(cp, Config{Addr: "127.0.0.1:6379"}, nil)
	assert.Error(t, err).Nil()
	_ = c.Close()

	// A provided Driver bean override is used as-is.
	fd := &fakeDriver{}
	c, err = newClient(cp, Config{Addr: "127.0.0.1:6379"}, fd)
	assert.Error(t, err).Nil()
	_ = c.Close()
	assert.That(t, fd.called).True()
}
