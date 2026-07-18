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
	"net"
	"testing"
	"time"

	"go-spring.org/stdlib/testing/assert"
)

func TestLiveDialer_ColdStartResolve(t *testing.T) {
	d := newStaticDiscovery()
	d.set("svc", Endpoint{Addr: "10.0.0.1:6379"}, Endpoint{Addr: "10.0.0.2:6379"})

	ld, err := NewLiveDialer(context.Background(), d, "svc")
	assert.Error(t, err).Nil()
	defer ld.Stop()

	assert.Slice(t, addrsOf(ld.Endpoints())).Equal([]string{"10.0.0.1:6379", "10.0.0.2:6379"})
}

func TestLiveDialer_PickRoundRobin(t *testing.T) {
	d := newStaticDiscovery()
	d.set("svc", Endpoint{Addr: "a:1"}, Endpoint{Addr: "b:2"}, Endpoint{Addr: "c:3"})

	ld, err := NewLiveDialer(context.Background(), d, "svc")
	assert.Error(t, err).Nil()
	defer ld.Stop()

	var got []string
	for i := 0; i < 6; i++ {
		ep, err := ld.Pick()
		assert.Error(t, err).Nil()
		got = append(got, ep.Addr)
	}
	// Two full cycles over the three endpoints, in order.
	assert.Slice(t, got).Equal([]string{"a:1", "b:2", "c:3", "a:1", "b:2", "c:3"})
}

func TestLiveDialer_PickPrefersHealthy(t *testing.T) {
	d := newStaticDiscovery()
	d.set("svc",
		Endpoint{Addr: "down:1", Healthy: false},
		Endpoint{Addr: "up:2", Healthy: true},
	)

	ld, err := NewLiveDialer(context.Background(), d, "svc")
	assert.Error(t, err).Nil()
	defer ld.Stop()

	for i := 0; i < 4; i++ {
		ep, err := ld.Pick()
		assert.Error(t, err).Nil()
		assert.That(t, ep.Addr).Equal("up:2")
	}
}

func TestLiveDialer_PickFallsBackWhenNoneHealthy(t *testing.T) {
	d := newStaticDiscovery()
	// No endpoint marked healthy — all are eligible so discovery still works.
	d.set("svc", Endpoint{Addr: "x:1"}, Endpoint{Addr: "y:2"})

	ld, err := NewLiveDialer(context.Background(), d, "svc")
	assert.Error(t, err).Nil()
	defer ld.Stop()

	ep, err := ld.Pick()
	assert.Error(t, err).Nil()
	assert.That(t, ep.Addr == "x:1" || ep.Addr == "y:2").True()
}

func TestLiveDialer_PickEmpty(t *testing.T) {
	d := newStaticDiscovery()
	d.set("svc") // empty

	ld, err := NewLiveDialer(context.Background(), d, "svc")
	assert.Error(t, err).Nil()
	defer ld.Stop()

	_, err = ld.Pick()
	assert.Error(t, err).Matches("no endpoints")
}

func TestLiveDialer_WatchUpdatesSnapshot(t *testing.T) {
	d := newStaticDiscovery()
	d.set("svc", Endpoint{Addr: "old:1"})

	ld, err := NewLiveDialer(context.Background(), d, "svc")
	assert.Error(t, err).Nil()
	defer ld.Stop()

	d.Update("svc", Endpoint{Addr: "new:1"}, Endpoint{Addr: "new:2"})

	// watchLoop applies the snapshot asynchronously; poll briefly.
	assert.That(t, waitFor(func() bool {
		return len(ld.Endpoints()) == 2
	})).True()
	assert.Slice(t, addrsOf(ld.Endpoints())).Equal([]string{"new:1", "new:2"})
}

func TestLiveDialer_DialConnectsToLiveEndpoint(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	assert.Error(t, err).Nil()
	defer ln.Close()

	d := newStaticDiscovery()
	d.set("svc", Endpoint{Addr: ln.Addr().String(), Healthy: true})

	ld, err := NewLiveDialer(context.Background(), d, "svc")
	assert.Error(t, err).Nil()
	defer ld.Stop()

	// Both dialer shapes ignore the passed addr and reach the live endpoint.
	c1, err := ld.Dial(context.Background(), "ignored:0")
	assert.Error(t, err).Nil()
	assert.That(t, c1).NotNil()
	c1.Close()

	c2, err := ld.DialContext(context.Background(), "tcp", "ignored:0")
	assert.Error(t, err).Nil()
	assert.That(t, c2).NotNil()
	c2.Close()
}

func TestLiveDialer_StopIdempotent(t *testing.T) {
	d := newStaticDiscovery()
	d.set("svc", Endpoint{Addr: "a:1"})

	ld, err := NewLiveDialer(context.Background(), d, "svc")
	assert.Error(t, err).Nil()

	assert.Error(t, ld.Stop()).Nil()
	assert.Error(t, ld.Stop()).Nil()
}

func addrsOf(eps []Endpoint) []string {
	out := make([]string, len(eps))
	for i, ep := range eps {
		out[i] = ep.Addr
	}
	return out
}

func waitFor(cond func() bool) bool {
	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		if cond() {
			return true
		}
		time.Sleep(5 * time.Millisecond)
	}
	return cond()
}
