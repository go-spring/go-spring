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
)

// TestDefaultDriverFallback proves assembleTransport falls back to the bundled
// DefaultDriver when no Driver bean is provided (nil). Building the transport
// needs no live server — it only assembles the RoundTripper.
func TestDefaultDriverFallback(t *testing.T) {
	rt, _, err := assembleTransport(nil, "x", Config{Addr: "10.0.0.1:8080"}, nil)
	if err != nil {
		t.Fatalf("assembleTransport with nil driver failed: %v", err)
	}
	if rt == nil {
		t.Fatal("expected a transport from the DefaultDriver fallback")
	}
}

// recordingDriver is the "ADD to the default" shape; it delegates to
// DefaultDriver and records that it was the active assembly driver.
type recordingDriver struct {
	DefaultDriver
	called bool
}

func (d *recordingDriver) CreateTransport(ctx context.Context, name string, c Config) (http.RoundTripper, func() error, error) {
	d.called = true
	return d.DefaultDriver.CreateTransport(ctx, name, c)
}

// TestCustomDriverUsed proves a provided Driver bean is the one assembleTransport
// dispatches through, replacing the internal DefaultDriver fallback.
func TestCustomDriverUsed(t *testing.T) {
	drv := &recordingDriver{}
	rt, _, err := assembleTransport(nil, "x", Config{Addr: "10.0.0.1:8080"}, drv)
	if err != nil {
		t.Fatalf("assembleTransport failed: %v", err)
	}
	if rt == nil {
		t.Fatal("expected a transport")
	}
	if !drv.called {
		t.Fatal("expected assembleTransport to dispatch through the provided Driver bean")
	}
}
