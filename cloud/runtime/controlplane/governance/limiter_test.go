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

package governance

import (
	"context"
	"errors"
	"testing"

	"go-spring.org/cloud/runtime/dataplane/inbound/admission"
	"go-spring.org/cloud/runtime/dataplane/kernel/ratelimit"
)

// fakeStore is a ratelimit.ClusterLimiter whose verdict the test decides.
type fakeStore struct {
	admit bool
	err   error
	calls int
	scope string
}

func (s *fakeStore) Allow(_ context.Context, scope string, _ ratelimit.Spec, _ int) (bool, error) {
	s.calls++
	s.scope = scope
	return s.admit, s.err
}

// TestInboundUsesClusterLimiter pins the whole inbound rate-limit path: the
// center's CenterOption reaches the guard manager, the guard charges the shared
// store, and the shared rate-limit sentinel surfaces as VerdictRateLimited.
func TestInboundUsesClusterLimiter(t *testing.T) {
	store := &fakeStore{admit: false}
	resClient, lb, fc, fs := newAuthorities()
	c, err := NewCenter(resClient, admission.NewManager(admission.WithClusterLimiter(store)), lb, fc, fs, pushSource(Config{Server: ServerConfig{Default: admission.Policy{RateLimit: 10}}}))
	if err != nil {
		t.Fatal(err)
	}

	g, err := c.guardServer.GuardFor(admission.Identity{System: "gin", Service: "gin:test"})
	if err != nil {
		t.Fatal(err)
	}
	r, done := g.Admit(context.Background(), admission.Request{})
	defer done()

	if r.Admitted() {
		t.Fatal("want the inbound rate limit to refuse the request")
	}
	if r.Verdict() != admission.VerdictRateLimited {
		t.Fatalf("want rate limited, got %v", r.Verdict())
	}
	if store.calls != 1 || store.scope != "gin:test" {
		t.Fatalf("store not charged: calls=%d scope=%q", store.calls, store.scope)
	}
}

// TestInboundStoreFailureAllows pins the failure policy: an unreachable counter
// backend must not turn into an outage.
func TestInboundStoreFailureAllows(t *testing.T) {
	store := &fakeStore{admit: true, err: errors.New("store down")}
	resClient, lb, fc, fs := newAuthorities()
	c, err := NewCenter(resClient, admission.NewManager(admission.WithClusterLimiter(store)), lb, fc, fs, pushSource(Config{Server: ServerConfig{Default: admission.Policy{RateLimit: 10}}}))
	if err != nil {
		t.Fatal(err)
	}

	g, err := c.guardServer.GuardFor(admission.Identity{System: "gin", Service: "gin:test"})
	if err != nil {
		t.Fatal(err)
	}
	r, done := g.Admit(context.Background(), admission.Request{})
	defer done()
	if !r.Admitted() {
		t.Fatalf("store failure must allow the request: %v", r.Err())
	}
}
