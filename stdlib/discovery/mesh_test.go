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

// panicDiscovery fails the moment it is resolved or watched. Mesh mode must
// never touch a backend, so any call here means the degradation leaked through.
type panicDiscovery struct{}

func (panicDiscovery) Resolve(context.Context, string) ([]Endpoint, error) {
	panic("mesh mode must not resolve the discovery backend")
}

func (panicDiscovery) Watch(context.Context, string) (Watcher, error) {
	panic("mesh mode must not watch the discovery backend")
}

func TestMeshMode_Toggle(t *testing.T) {
	t.Cleanup(func() { SetMeshMode(false) })

	assert.That(t, MeshMode()).False()
	SetMeshMode(true)
	assert.That(t, MeshMode()).True()
	SetMeshMode(false)
	assert.That(t, MeshMode()).False()
}

func TestNewLiveDialer_MeshDegradesToStableEndpoint(t *testing.T) {
	t.Cleanup(func() { SetMeshMode(false) })
	SetMeshMode(true)

	// Even a backend that panics on use is fine: mesh mode never calls it.
	ld, err := NewLiveDialer(context.Background(), panicDiscovery{}, "user-svc")
	assert.Error(t, err).Nil()
	defer ld.Stop()

	// One stable endpoint (the service name), never rotating.
	eps := ld.Endpoints()
	assert.Slice(t, addrsOf(eps)).Equal([]string{"user-svc"})
	for range 5 {
		ep, err := ld.Pick()
		assert.Error(t, err).Nil()
		assert.String(t, ep.Addr).Equal("user-svc")
	}

	// Stop is a no-op with no watch and must not panic.
	assert.Error(t, ld.Stop()).Nil()
}

func TestNewClientDialer_MeshSkipsBackendLookup(t *testing.T) {
	t.Cleanup(func() { SetMeshMode(false) })
	SetMeshMode(true)

	// "no-such-backend" is not registered; in mesh mode NewClientDialer must not
	// look it up, so this succeeds rather than erroring on a missing backend.
	ld, err := NewClientDialer(context.Background(), "no-such-backend", "order-svc")
	assert.Error(t, err).Nil()
	defer ld.Stop()
	assert.Slice(t, addrsOf(ld.Endpoints())).Equal([]string{"order-svc"})
}

func TestNewClientDialer_NormalModeResolvesBackend(t *testing.T) {
	d := newStaticDiscovery()
	d.set("svc", Endpoint{Addr: "10.0.0.1:6379"}, Endpoint{Addr: "10.0.0.2:6379"})
	Register("mesh-test-backend", d)

	ld, err := NewClientDialer(context.Background(), "mesh-test-backend", "svc")
	assert.Error(t, err).Nil()
	defer ld.Stop()
	assert.Slice(t, addrsOf(ld.Endpoints())).Equal([]string{"10.0.0.1:6379", "10.0.0.2:6379"})
}
