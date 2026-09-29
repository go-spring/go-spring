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
	"testing"

	"go-spring.org/cloud/governance/fault"
	"go-spring.org/cloud/governance/resilience"
)

// TestConfig_DirectionBlocks guards the shape of the document: each direction's
// config lives in its own block and is preserved on the Config (so the center can
// push it into the injector), and the two blocks do not bleed into one another.
// This is the structural anchor of the client/server split.
func TestConfig_DirectionBlocks(t *testing.T) {
	// Zero Config: both fault sides disabled, resilience disabled.
	var zero Config
	if zero.Client.Fault.Enabled || zero.Server.Fault.Enabled {
		t.Fatal("zero Config: both fault sides should be disabled")
	}
	c := newTestCenter(zero)
	if p := c.clientPolicyFor("x"); p.AttemptTimeout != 0 {
		t.Fatalf("zero config policyFor: want no timeout, got %v", p.AttemptTimeout)
	}
	if a := c.serverPolicyFor("x"); !a.IsZero() {
		t.Fatalf("zero config serverPolicyFor: want a zero admission, got %+v", a)
	}

	// One fault config per side: each is preserved, and they stay independent.
	cfg := Config{
		Enabled: true,
		Client: ClientConfig{
			Default: ClientDefaultPolicy{ClientPolicy: resilience.ClientPolicy{AttemptTimeout: dur(100)}},
			Fault:   fault.Config{Enabled: true, Rate: 0.5, Error: "generic"},
		},
		Server: ServerConfig{
			Fault: fault.Config{Enabled: true, Rate: 1, Error: "timeout"},
		},
	}
	if cf := cfg.Client.Fault; !cf.Enabled || cf.Rate != 0.5 || cf.Error != "generic" {
		t.Fatalf("ClientConfig.Fault not preserved: %+v", cf)
	}
	if sf := cfg.Server.Fault; !sf.Enabled || sf.Rate != 1 || sf.Error != "timeout" {
		t.Fatalf("ServerConfig.Fault not preserved: %+v", sf)
	}
	cc := newTestCenter(cfg)
	if p := cc.clientPolicyFor("redis:cache"); p.AttemptTimeout != dur(100) {
		t.Fatalf("policyFor with Fault set: want timeout 100ms, got %v", p.AttemptTimeout)
	}
	// An outbound-only document leaves admission at zero: the server side does not
	// inherit the client's knobs.
	if a := cc.serverPolicyFor("gin:0.0.0.0:8080"); !a.IsZero() {
		t.Fatalf("client-only document must leave admission zero, got %+v", a)
	}
}

// TestDispatch_RejectsDuplicateRules covers the duplicate-label guard: a
// document naming the same service twice is rejected whole — the previous
// snapshot keeps serving — while several empty labels stay harmless.
func TestDispatch_RejectsDuplicateRules(t *testing.T) {
	good := Config{
		Enabled: true,
		Client: ClientConfig{
			Rules: []ClientRule{{Service: "redis:cache"}, {Service: "redis:session"}},
		},
	}
	c, _, _, _ := newTestCenterWith(good, nil)

	bad := Config{
		Enabled: true,
		Client: ClientConfig{
			Rules: []ClientRule{
				{Service: "redis:cache", ClientPolicy: resilience.ClientPolicy{AttemptTimeout: dur(50)}},
				{Service: "redis:cache", ClientPolicy: resilience.ClientPolicy{AttemptTimeout: dur(999)}},
			},
		},
	}
	if err := c.dispatch(bad); err == nil {
		t.Fatal("dispatch must reject a document with duplicate service labels")
	}
	// The previous snapshot keeps serving: the good timeout, not either duplicate.
	if p := c.clientPolicyFor("redis:cache"); p.AttemptTimeout != 0 {
		t.Fatalf("rejected push must not leak into the serving snapshot: got timeout %v", p.AttemptTimeout)
	}

	// Empty labels match nothing and are not duplicates of one another.
	withEmpty := Config{
		Enabled: true,
		Client:  ClientConfig{Rules: []ClientRule{{}, {Service: "redis:cache"}}},
	}
	if err := c.dispatch(withEmpty); err != nil {
		t.Fatalf("empty service labels must be allowed: %v", err)
	}
}
