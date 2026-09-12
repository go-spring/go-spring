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

package StarterNeo4j

import (
	"errors"
	"sync"
	"testing"

	"github.com/neo4j/neo4j-go-driver/v5/neo4j"
	"go-spring.org/cloud/discovery"
	"go-spring.org/stdlib/testing/assert"
)

// fakeResolver is a mutable discovery.Resolver: a test sets the endpoint set it
// should report, which is what a naming service does when membership changes.
type fakeResolver struct {
	mu  sync.Mutex
	eps []discovery.Endpoint
	err error
}

func (f *fakeResolver) set(addrs ...string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.err = nil
	f.eps = f.eps[:0]
	for _, a := range addrs {
		f.eps = append(f.eps, discovery.Endpoint{Addr: a, Healthy: true, Weight: 1})
	}
}

func (f *fakeResolver) fail(err error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.err = err
}

func (f *fakeResolver) resolve() ([]discovery.Endpoint, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.err != nil {
		return nil, f.err
	}
	return append([]discovery.Endpoint(nil), f.eps...), nil
}

// hostPorts renders a resolver answer as "host:port" strings for comparison.
func hostPorts(addrs []neo4j.ServerAddress) []string {
	out := make([]string, 0, len(addrs))
	for _, a := range addrs {
		out = append(out, a.Hostname()+":"+a.Port())
	}
	return out
}

// TestLiveRouterAddressesFollowsMembership covers the recovery this hook buys a
// routing-mode client: when the driver cannot reach the address it was dialed
// with, it asks for routers again — and gets the naming service's current view,
// not the boot-time one.
func TestLiveRouterAddressesFollowsMembership(t *testing.T) {
	f := &fakeResolver{}
	f.set("10.0.0.1:7687")
	current := neo4j.NewServerAddress("10.0.0.1", "7687")
	hook := liveRouterAddresses(f.resolve)
	assert.That(t, hostPorts(hook(current))).Equal([]string{"10.0.0.1:7687"})

	// The dialed host is gone; discovery has moved on.
	f.set("10.0.0.2:7687", "10.0.0.3:7687")
	assert.That(t, hostPorts(hook(current))).Equal([]string{"10.0.0.2:7687", "10.0.0.3:7687"})
}

// TestLiveRouterAddressesFallsBackToCurrent pins the failure mode: a registry
// hiccup must leave the driver with the address it already had, never with an
// empty router list it would then fail on.
func TestLiveRouterAddressesFallsBackToCurrent(t *testing.T) {
	current := neo4j.NewServerAddress("10.0.0.1", "7687")
	f := &fakeResolver{}
	hook := liveRouterAddresses(f.resolve)

	f.set() // membership reported as empty
	assert.That(t, hostPorts(hook(current))).Equal([]string{"10.0.0.1:7687"})

	f.fail(errors.New("registry down"))
	assert.That(t, hostPorts(hook(current))).Equal([]string{"10.0.0.1:7687"})

	// An address that is not host:port cannot become a router either.
	f.set("not-a-hostport")
	assert.That(t, hostPorts(hook(current))).Equal([]string{"10.0.0.1:7687"})
}
