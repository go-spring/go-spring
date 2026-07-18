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
	"fmt"
	"net"
	"sync"
	"sync/atomic"
)

// LiveDialer turns a [Discovery] backend into a net-style dialer that always
// connects to a currently-live endpoint of a service.
//
// It is the shared piece every infrastructure-client starter reuses: resolve
// the service name once at cold start, then keep a background Watch running so
// the endpoint snapshot stays fresh. Clients inject [LiveDialer.DialContext] as
// their dialer and ignore the static address; combined with a bounded
// connection lifetime (e.g. Redis ConnMaxLifetime) old connections retire and
// new ones land on the updated addresses — a smooth switch without rebuilding
// the client.
type LiveDialer struct {
	name    string
	dialer  net.Dialer
	watcher Watcher

	eps  atomic.Pointer[[]Endpoint]
	next atomic.Uint64

	stopOnce sync.Once
}

// NewLiveDialer resolves name once through d, starts watching it in the
// background, and returns a dialer over the live endpoints. The caller must
// call Stop to release the watch. Any options applied to the returned
// LiveDialer.Dialer (timeout, keep-alive, ...) are used for every dial.
func NewLiveDialer(ctx context.Context, d Discovery, name string) (*LiveDialer, error) {
	eps, err := d.Resolve(ctx, name)
	if err != nil {
		return nil, fmt.Errorf("discovery: resolve %q: %w", name, err)
	}
	w, err := d.Watch(ctx, name)
	if err != nil {
		return nil, fmt.Errorf("discovery: watch %q: %w", name, err)
	}
	ld := &LiveDialer{name: name, watcher: w}
	ld.eps.Store(&eps)
	go ld.watchLoop()
	return ld, nil
}

// watchLoop applies every snapshot the watcher yields until it stops or errors.
func (ld *LiveDialer) watchLoop() {
	for {
		eps, err := ld.watcher.Next()
		if err != nil {
			return
		}
		ld.eps.Store(&eps)
	}
}

// Endpoints returns the current endpoint snapshot.
func (ld *LiveDialer) Endpoints() []Endpoint {
	return *ld.eps.Load()
}

// Pick selects one live endpoint using weight-free round-robin. Endpoints
// marked Healthy are preferred; if none are marked healthy (backends that do
// not track health), all endpoints are considered eligible so discovery still
// works. It errors only when the service currently has no endpoints at all.
func (ld *LiveDialer) Pick() (Endpoint, error) {
	eps := *ld.eps.Load()
	if len(eps) == 0 {
		return Endpoint{}, fmt.Errorf("discovery: no endpoints for %q", ld.name)
	}

	eligible := eps[:0:0]
	for _, ep := range eps {
		if ep.Healthy {
			eligible = append(eligible, ep)
		}
	}
	if len(eligible) == 0 {
		eligible = eps
	}

	i := ld.next.Add(1) - 1
	return eligible[int(i%uint64(len(eligible)))], nil
}

// DialContext dials a live endpoint, ignoring addr. Its signature matches the
// dialer hooks of common clients (e.g. redis.Options.Dialer, pgx DialFunc).
func (ld *LiveDialer) DialContext(ctx context.Context, network, _ string) (net.Conn, error) {
	ep, err := ld.Pick()
	if err != nil {
		return nil, err
	}
	return ld.dialer.DialContext(ctx, network, ep.Addr)
}

// Dial is the two-argument form of DialContext for drivers whose dialer hook
// omits the network (e.g. go-sql-driver/mysql DialContextFunc, ClickHouse
// Options.DialContext). It dials a live endpoint over TCP, ignoring addr.
func (ld *LiveDialer) Dial(ctx context.Context, _ string) (net.Conn, error) {
	return ld.DialContext(ctx, "tcp", "")
}

// Stop ends the background watch. It is safe to call more than once.
func (ld *LiveDialer) Stop() error {
	var err error
	ld.stopOnce.Do(func() {
		err = ld.watcher.Stop()
	})
	return err
}
