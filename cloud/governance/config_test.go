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
	"testing"

	"go-spring.org/cloud/fault"
	"go-spring.org/cloud/resilience"
)

// TestConfig_DirectionBlocks guards the shape of the document: each direction's
// config lives in its own block and is preserved on the Config (so the center can
// push it into the injector), and the two blocks do not bleed into one another.
// This is the structural anchor of the client/server split.
func TestConfig_DirectionBlocks(t *testing.T) {
	// Zero Config: both fault sides inert, resilience disabled.
	var zero Config
	if !zero.Client.Fault.Inert() || !zero.Server.Fault.Inert() {
		t.Fatal("zero Config: both fault sides should be inert")
	}
	c := newTestCenter(zero)
	if p := c.resClient.ClientPolicyFor("x"); p.Timeout != 0 {
		t.Fatalf("zero config policyFor: want no timeout, got %v", p.Timeout)
	}
	if a := c.GuardServer().PolicyFor("x"); !a.IsZero() {
		t.Fatalf("zero config serverPolicyFor: want a zero inbound, got %+v", a)
	}

	// One fault config per side: each is preserved, and they stay independent.
	cfg := Config{
		Client: ClientConfig{
			Default: ClientDefaultPolicy{ClientPolicy: resilience.ClientPolicy{Timeout: dur(100)}},
			Fault:   fault.Config{Rules: []fault.Rule{{Rate: 0.5, Error: "generic"}}},
		},
		Server: ServerConfig{
			Fault: fault.Config{Rules: []fault.Rule{{Rate: 1, Error: "timeout"}}},
		},
	}
	if cf := cfg.Client.Fault; len(cf.Rules) != 1 ||
		cf.Rules[0].Rate != 0.5 || cf.Rules[0].Error != "generic" {
		t.Fatalf("ClientConfig.Fault not preserved: %+v", cf)
	}
	if sf := cfg.Server.Fault; len(sf.Rules) != 1 ||
		sf.Rules[0].Rate != 1 || sf.Rules[0].Error != "timeout" {
		t.Fatalf("ServerConfig.Fault not preserved: %+v", sf)
	}
	cc := newTestCenter(cfg)
	if p := cc.resClient.ClientPolicyFor("redis:cache"); p.Timeout != dur(100) {
		t.Fatalf("policyFor with Fault set: want timeout 100ms, got %v", p.Timeout)
	}
	// An outbound-only document leaves inbound at zero: the server side does not
	// inherit the client's knobs.
	if a := cc.GuardServer().PolicyFor("gin:0.0.0.0:8080"); !a.IsZero() {
		t.Fatalf("client-only document must leave inbound zero, got %+v", a)
	}
}

// TestDispatch_RejectsDuplicateRules covers the duplicate-label guard: a
// document naming the same service twice is rejected whole — the previous
// snapshot keeps serving — while several empty labels stay harmless.
func TestDispatch_RejectsDuplicateRules(t *testing.T) {
	good := Config{
		Client: ClientConfig{
			Rules: []ClientRule{{Service: "redis:cache"}, {Service: "redis:session"}},
		},
	}
	c, _, _, _, _ := newTestCenterWith(good)

	bad := Config{
		Client: ClientConfig{
			Rules: []ClientRule{
				{Service: "redis:cache", ClientPolicy: resilience.ClientPolicy{Timeout: dur(50)}},
				{Service: "redis:cache", ClientPolicy: resilience.ClientPolicy{Timeout: dur(999)}},
			},
		},
	}
	if err := c.dispatch(context.Background(), bad); err == nil {
		t.Fatal("dispatch must reject a document with duplicate service labels")
	}
	// The previous snapshot keeps serving: the good timeout, not either duplicate.
	if p := c.resClient.ClientPolicyFor("redis:cache"); p.Timeout != 0 {
		t.Fatalf("rejected push must not leak into the serving snapshot: got timeout %v", p.Timeout)
	}

	// Empty labels match nothing and are not duplicates of one another.
	withEmpty := Config{
		Client: ClientConfig{Rules: []ClientRule{{}, {Service: "redis:cache"}}},
	}
	if err := c.dispatch(context.Background(), withEmpty); err != nil {
		t.Fatalf("empty service labels must be allowed: %v", err)
	}
}
