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

	"go-spring.org/cloud/chain"
	"go-spring.org/cloud/guard"
	"go-spring.org/cloud/observability"
	"go-spring.org/cloud/resilience"
)

// fakeStore is a resilience.ClusterLimiter whose verdict the test decides.
type fakeStore struct {
	err   error
	calls int
	scope string
}

func (s *fakeStore) Allow(_ context.Context, scope string, _ resilience.RateSpec, _ int) error {
	s.calls++
	s.scope = scope
	return s.err
}

// TestInboundUsesClusterLimiter pins the whole inbound rate-limit path: the
// center's CenterOption reaches the guard manager, the guard charges the shared
// store, and the store's chain sentinel is translated into guard's own so the
// refusal surfaces as VerdictRateLimited.
func TestInboundUsesClusterLimiter(t *testing.T) {
	store := &fakeStore{err: chain.ErrRateLimited}
	resClient, lb, fc, fs := newAuthorities()
	c, err := NewCenter(resClient, lb, fc, fs, nil,
		NewPushSource(Config{Server: ServerConfig{Default: guard.Policy{RateLimit: 10}}}),
		WithClusterLimiter(guardLimiter{inner: store}))
	if err != nil {
		t.Fatal(err)
	}

	in := c.InboundFor("gin", "gin:test")
	err = in.Run(context.Background(), observability.ServerOperation{}, func(context.Context) error { return nil })

	var ref *Refusal
	if !errors.As(err, &ref) {
		t.Fatalf("want a refusal, got %v", err)
	}
	if ref.Verdict() != guard.VerdictRateLimited {
		t.Fatalf("want rate limited, got %v", ref.Verdict())
	}
	if store.calls != 1 || store.scope != "gin:test" {
		t.Fatalf("store not charged: calls=%d scope=%q", store.calls, store.scope)
	}
}

// TestInboundStoreFailureAllows pins the failure policy: an unreachable counter
// backend must not turn into an outage.
func TestInboundStoreFailureAllows(t *testing.T) {
	store := &fakeStore{err: errors.New("store down")}
	resClient, lb, fc, fs := newAuthorities()
	c, err := NewCenter(resClient, lb, fc, fs, nil,
		NewPushSource(Config{Server: ServerConfig{Default: guard.Policy{RateLimit: 10}}}),
		WithClusterLimiter(guardLimiter{inner: store}))
	if err != nil {
		t.Fatal(err)
	}

	ran := false
	err = c.InboundFor("gin", "gin:test").Run(context.Background(), observability.ServerOperation{}, func(context.Context) error {
		ran = true
		return nil
	})
	if err != nil {
		t.Fatalf("store failure must allow the request: %v", err)
	}
	if !ran {
		t.Fatal("handler did not run")
	}
}
