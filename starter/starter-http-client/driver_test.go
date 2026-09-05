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

package StarterHTTPClient

import (
	"context"
	"net/http"
	"testing"

	"go-spring.org/stdlib/testing/assert"
)

func TestDriverRegistry(t *testing.T) {
	// The bundled default driver is registered at init.
	_, ok := driverRegistry["default"]
	assert.That(t, ok).True()

	// Custom drivers register by name; duplicate names panic.
	RegisterDriver("test-add", addDriver{})
	assert.Panic(t, func() { RegisterDriver("test-add", addDriver{}) }, "already registered")
}

// addDriver is the "ADD to the default" shape: it delegates to DefaultDriver
// and wraps the returned transport with an extra header.
type addDriver struct {
	DefaultDriver
	extraSeen *bool
}

func (d addDriver) CreateTransport(ctx context.Context, name string, c Config) (http.RoundTripper, func() error, error) {
	rt, closeFn, err := d.DefaultDriver.CreateTransport(ctx, name, c)
	if err != nil {
		return nil, nil, err
	}
	return wrapHeader(rt, "X-Custom", d.extraSeen), closeFn, nil
}

func wrapHeader(next http.RoundTripper, key string, seen *bool) http.RoundTripper {
	return roundTripFunc(func(req *http.Request) (*http.Response, error) {
		if req.Header.Get(key) != "" {
			*seen = true
		}
		return next.RoundTrip(req)
	})
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) { return f(req) }

func TestUnknownDriverFailsFast(t *testing.T) {
	_, _, err := assembleTransport(nil, "x", Config{
		Addr: "10.0.0.1:8080", Driver: "no-such",
	})
	assert.Error(t, err).Matches("unknown driver")
}
