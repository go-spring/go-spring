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

package loadbalance

import (
	"testing"

	"go-spring.org/stdlib/testing/assert"
)

func TestRegistry(t *testing.T) {
	for _, name := range []string{RoundRobin, LeastConn, ConsistentHash, Weighted, ZoneAware, Random, P2C} {
		b, err := New(name, Config{})
		assert.Error(t, err).Nil()
		assert.That(t, b).NotNil()
	}

	_, err := New("does-not-exist", Config{})
	assert.Error(t, err).Matches("no strategy registered")

	assert.Panic(t, func() { Register("", func(Config) (Balancer, error) { return nil, nil }) }, "empty name")
	assert.Panic(t, func() { Register("x", nil) }, "nil factory")
	assert.Panic(t, func() { Register(RoundRobin, func(Config) (Balancer, error) { return NewRoundRobin(), nil }) }, "already registered")
}

func TestConfigPartition(t *testing.T) {
	// A parameter aimed at another strategy is rejected, not silently dropped.
	_, err := New(LeastConn, Config{Replicas: 200})
	assert.Error(t, err).Matches("ignored by this strategy")

	_, err = New(ConsistentHash, Config{ZoneKey: "zone"})
	assert.Error(t, err).Matches("ignored by this strategy")

	// The strategy's own fields pass; the zero config passes everywhere.
	for _, name := range []string{RoundRobin, LeastConn, ConsistentHash, Weighted, ZoneAware, Random, P2C} {
		if _, err := New(name, Config{}); err != nil {
			t.Fatalf("zero config rejected for %s: %v", name, err)
		}
	}
	if _, err := New(ConsistentHash, Config{Replicas: 200}); err != nil {
		t.Fatal(err)
	}

	// zone_aware delegate errors: unknown name, and self-delegation.
	_, err = New(ZoneAware, Config{Delegate: "nope"})
	assert.Error(t, err).Matches("zone_aware delegate")
	_, err = New(ZoneAware, Config{Delegate: ZoneAware})
	assert.Error(t, err).Matches("cannot delegate to itself")
}
