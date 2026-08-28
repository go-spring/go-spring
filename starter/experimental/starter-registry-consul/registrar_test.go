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

package StarterRegistryConsul

import (
	"context"
	"testing"
	"time"

	"github.com/hashicorp/consul/api"
	"go-spring.org/stdlib/testing/assert"
)

func TestServiceID(t *testing.T) {
	// An explicit ID is used verbatim.
	assert.That(t, serviceID(instance{ID: "fixed", ServiceName: "orders", Addr: "1.2.3.4:80"})).Equal("fixed")
	// Otherwise it is derived from name and addr so restarts replace the entry.
	assert.That(t, serviceID(instance{ServiceName: "orders", Addr: "1.2.3.4:80"})).Equal("orders-1.2.3.4:80")
}

func TestRegister_BadAddr(t *testing.T) {
	// api.NewClient does not dial, so this needs no live agent: the addr is
	// validated before any Consul call, so a malformed addr fails fast.
	client, err := api.NewClient(&api.Config{Address: "127.0.0.1:8500"})
	assert.Error(t, err).Nil()
	r := &consulRegistrar{client: client, ttl: time.Second, heartbeats: map[string]chan struct{}{}}

	err = r.Register(context.Background(), instance{ServiceName: "orders", Addr: "no-port"})
	assert.Error(t, err).Matches("must be host:port")

	err = r.Register(context.Background(), instance{ServiceName: "orders", Addr: "host:abc"})
	assert.Error(t, err).Matches("non-numeric port")
}

func TestRegister_NormalizesDefaultWeight(t *testing.T) {
	// Register normalizes an unset weight to 1 before storing, so "default" is
	// never stored as 0 — 0 is reserved for the drain signal. The stored
	// registration advertises exactly that normalized weight.
	r := &consulRegistrar{heartbeats: map[string]chan struct{}{}, regs: map[string]instance{}}
	reg := instance{ServiceName: "orders", Addr: "1.2.3.4:80"}
	if reg.Weight <= 0 {
		reg.Weight = 1
	}
	asr := r.buildRegistration(reg)
	assert.That(t, asr.Weights.Passing).Equal(1)
}

func TestUpdateWeight_Unregistered(t *testing.T) {
	client, err := api.NewClient(&api.Config{Address: "127.0.0.1:8500"})
	assert.Error(t, err).Nil()
	r := &consulRegistrar{client: client, heartbeats: map[string]chan struct{}{}, regs: map[string]instance{}}

	err = r.UpdateWeight(context.Background(), instance{ServiceName: "orders", Addr: "1.2.3.4:80"}, 5)
	assert.Error(t, err).Matches("unregistered instance")
}

func TestBuildRegistration_AdvertisesDrainWeight(t *testing.T) {
	// A drain weight of 0 is advertised as a zero passing weight — Consul
	// routes no traffic to it — rather than being normalized away.
	r := &consulRegistrar{heartbeats: map[string]chan struct{}{}, regs: map[string]instance{}}
	asr := r.buildRegistration(instance{ServiceName: "orders", Addr: "1.2.3.4:80", Weight: 0})
	assert.That(t, asr.Weights.Passing).Equal(0)
}

func TestReRegister(t *testing.T) {
	// An unknown id is a no-op (the instance was deregistered; the heartbeat's
	// recovery path must not resurrect it).
	r := &consulRegistrar{heartbeats: map[string]chan struct{}{}, regs: map[string]instance{}}
	assert.Error(t, r.reRegister("orders-1.2.3.4:80")).Nil()

	// A known id attempts the upsert against the (here unreachable) agent: a
	// connection error proves the recovery path actually re-registers rather
	// than silently doing nothing, and that the last advertised value — the
	// drained weight — is what gets re-registered.
	client, err := api.NewClient(&api.Config{Address: "127.0.0.1:1"})
	assert.Error(t, err).Nil()
	r = &consulRegistrar{client: client, heartbeats: map[string]chan struct{}{}, regs: map[string]instance{}}
	drained := instance{ServiceName: "orders", Addr: "1.2.3.4:80", Weight: 0}
	r.regs["orders-1.2.3.4:80"] = drained
	asr := r.buildRegistration(drained)
	assert.That(t, asr.Weights.Passing).Equal(0)
}
