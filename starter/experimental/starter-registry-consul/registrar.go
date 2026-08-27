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
	"net"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/hashicorp/consul/api"
	"go-spring.org/log"
	"go-spring.org/stdlib/errutil"
)

// instance is the instance this process advertises to the Consul registry. It is
// the local write-side value built from RegistrationConfig in Server.Run; the
// crash-safety contract every registry starter follows lives in starter/DESIGN
// §3 (Register must self-renew so correctness never depends on Deregister).
type instance struct {
	ServiceName string
	ID          string
	Addr        string
	Weight      int
	Metadata    map[string]string
}

// consulRegistrar publishes instances to a Consul agent and keeps each one live
// by passing its TTL health check on a background heartbeat until Deregister.
type consulRegistrar struct {
	client                  *api.Client
	ttl                     time.Duration
	deregisterCriticalAfter time.Duration

	mu         sync.Mutex
	heartbeats map[string]chan struct{} // service ID -> heartbeat stop channel
	regs       map[string]instance     // service ID -> last registered value
}

// newConsulRegistrar builds a registrar backed by a Consul client for c.
func newConsulRegistrar(c ConsulConfig) (*consulRegistrar, error) {
	client, err := api.NewClient(&api.Config{
		Address:    c.Address,
		Scheme:     c.Scheme,
		Datacenter: c.Datacenter,
		Token:      c.Token,
		Namespace:  c.Namespace,
	})
	if err != nil {
		log.Errorf(context.Background(), starterTag, "create consul client for address=%s failed: %v", c.Address, err)
		return nil, err
	}
	return &consulRegistrar{
		client:                  client,
		ttl:                     c.TTL,
		deregisterCriticalAfter: c.DeregisterCriticalAfter,
		heartbeats:              map[string]chan struct{}{},
		regs:                    map[string]instance{},
	}, nil
}

// serviceID returns the Consul service instance id: the caller-supplied ID, or a
// stable one derived from the service name and advertised address.
func serviceID(reg instance) string {
	if reg.ID != "" {
		return reg.ID
	}
	return reg.ServiceName + "-" + reg.Addr
}

// Register publishes reg with a TTL health check, passes the check immediately
// so the instance is healthy without waiting a full TTL, then keeps it passing
// on a background heartbeat until Deregister.
func (r *consulRegistrar) Register(_ context.Context, reg instance) error {
	// An unset (or misconfigured negative) weight is normalized at write time
	// so "default" is never stored as 0 — 0 is reserved for the runtime drain
	// signal, only reachable through UpdateWeight.
	if reg.Weight <= 0 {
		reg.Weight = 1
	}
	if _, _, err := net.SplitHostPort(reg.Addr); err != nil {
		return errutil.Explain(err, "registry-consul: addr %q must be host:port", reg.Addr)
	}
	if _, err := strconv.Atoi(reg.Addr[strings.LastIndex(reg.Addr, ":")+1:]); err != nil {
		return errutil.Explain(err, "registry-consul: addr %q has a non-numeric port", reg.Addr)
	}
	id := serviceID(reg)
	checkID := "service:" + id
	if err := r.client.Agent().ServiceRegister(r.buildRegistration(reg)); err != nil {
		return errutil.Explain(err, "registry-consul: register %q", reg.ServiceName)
	}
	_ = r.client.Agent().UpdateTTL(checkID, "", api.HealthPassing)

	stop := make(chan struct{})
	r.mu.Lock()
	// Re-registering the same instance refreshes it: retire the old heartbeat.
	if old, ok := r.heartbeats[id]; ok {
		close(old)
	}
	r.heartbeats[id] = stop
	r.regs[id] = reg
	r.mu.Unlock()

	go r.heartbeat(checkID, stop)
	return nil
}

// buildRegistration assembles the full Consul service registration for reg —
// the TTL check definition included — so Register and UpdateWeight both
// upsert the identical entry apart from the weight.
func (r *consulRegistrar) buildRegistration(reg instance) *api.AgentServiceRegistration {
	host, portStr, err := net.SplitHostPort(reg.Addr)
	if err != nil {
		host = reg.Addr
	}
	port, _ := strconv.Atoi(portStr)
	return &api.AgentServiceRegistration{
		ID:      serviceID(reg),
		Name:    reg.ServiceName,
		Address: host,
		Port:    port,
		Meta:    reg.Metadata,
		// Consul treats a passing weight of 0 as "no traffic", which is exactly
		// the drain signal; a positive weight is advertised as-is.
		Weights: &api.AgentWeights{Passing: reg.Weight, Warning: 1},
		Check: &api.AgentServiceCheck{
			CheckID:                        "service:" + serviceID(reg),
			TTL:                            r.ttl.String(),
			DeregisterCriticalServiceAfter: r.deregisterCriticalAfter.String(),
		},
	}
}

// UpdateWeight re-publishes reg's entry with a new weight. ServiceRegister is
// an upsert on the service ID, so the same TTL check (kept passing by the
// existing heartbeat goroutine) survives and subscribers see the new weight
// without any re-registration. A weight of 0 drains the instance — Consul
// routes no traffic to a zero-weight service. It fails if reg was never
// registered through Register.
func (r *consulRegistrar) UpdateWeight(_ context.Context, reg instance, weight int) error {
	id := serviceID(reg)
	r.mu.Lock()
	last, ok := r.regs[id]
	r.mu.Unlock()
	if !ok {
		return errutil.Explain(nil, "registry-consul: update weight for unregistered instance %q", id)
	}
	if weight < 0 {
		weight = 1
	}
	last.Weight = weight
	if err := r.client.Agent().ServiceRegister(r.buildRegistration(last)); err != nil {
		return errutil.Explain(err, "registry-consul: update weight %q", last.ServiceName)
	}
	r.mu.Lock()
	r.regs[id] = last
	r.mu.Unlock()
	return nil
}

// heartbeat re-passes the TTL check at half the TTL until stop is closed.
func (r *consulRegistrar) heartbeat(checkID string, stop <-chan struct{}) {
	interval := r.ttl / 2
	if interval <= 0 {
		interval = r.ttl
	}
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-stop:
			return
		case <-ticker.C:
			_ = r.client.Agent().UpdateTTL(checkID, "", api.HealthPassing)
		}
	}
}

// Deregister stops the heartbeat and removes the instance. It is idempotent:
// deregistering an instance that is not registered is a no-op that still asks
// Consul to drop the id (harmless if already gone).
func (r *consulRegistrar) Deregister(_ context.Context, reg instance) error {
	id := serviceID(reg)
	r.mu.Lock()
	if stop, ok := r.heartbeats[id]; ok {
		close(stop)
		delete(r.heartbeats, id)
	}
	delete(r.regs, id)
	r.mu.Unlock()
	if err := r.client.Agent().ServiceDeregister(id); err != nil {
		return errutil.Explain(err, "registry-consul: deregister %q", reg.ServiceName)
	}
	return nil
}
