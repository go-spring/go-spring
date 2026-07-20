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
	"sync"
	"time"

	"github.com/hashicorp/consul/api"
	"go-spring.org/spring/discovery"
	"go-spring.org/stdlib/errutil"
)

// consulRegistrar publishes instances to a Consul agent and keeps each one live
// by passing its TTL health check on a background heartbeat until Deregister.
// It implements discovery.Registrar.
type consulRegistrar struct {
	client                  *api.Client
	ttl                     time.Duration
	deregisterCriticalAfter time.Duration

	mu         sync.Mutex
	heartbeats map[string]chan struct{} // service ID -> heartbeat stop channel
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
		return nil, err
	}
	return &consulRegistrar{
		client:                  client,
		ttl:                     c.TTL,
		deregisterCriticalAfter: c.DeregisterCriticalAfter,
		heartbeats:              map[string]chan struct{}{},
	}, nil
}

// serviceID returns the Consul service instance id: the caller-supplied ID, or a
// stable one derived from the service name and advertised address.
func serviceID(reg discovery.Registration) string {
	if reg.ID != "" {
		return reg.ID
	}
	return reg.ServiceName + "-" + reg.Addr
}

// Register publishes reg with a TTL health check, passes the check immediately
// so the instance is healthy without waiting a full TTL, then keeps it passing
// on a background heartbeat until Deregister.
func (r *consulRegistrar) Register(_ context.Context, reg discovery.Registration) error {
	host, portStr, err := net.SplitHostPort(reg.Addr)
	if err != nil {
		return errutil.Explain(err, "registry-consul: addr %q must be host:port", reg.Addr)
	}
	port, err := strconv.Atoi(portStr)
	if err != nil {
		return errutil.Explain(err, "registry-consul: addr %q has a non-numeric port", reg.Addr)
	}

	id := serviceID(reg)
	checkID := "service:" + id
	asr := &api.AgentServiceRegistration{
		ID:      id,
		Name:    reg.ServiceName,
		Address: host,
		Port:    port,
		Meta:    reg.Metadata,
		Check: &api.AgentServiceCheck{
			CheckID:                        checkID,
			TTL:                            r.ttl.String(),
			DeregisterCriticalServiceAfter: r.deregisterCriticalAfter.String(),
		},
	}
	// Weight 0 in Consul means "no traffic", so only advertise a weight when the
	// caller set one; otherwise let Consul apply its default.
	if reg.Weight > 0 {
		asr.Weights = &api.AgentWeights{Passing: reg.Weight, Warning: 1}
	}
	if err := r.client.Agent().ServiceRegister(asr); err != nil {
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
	r.mu.Unlock()

	go r.heartbeat(checkID, stop)
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
func (r *consulRegistrar) Deregister(_ context.Context, reg discovery.Registration) error {
	id := serviceID(reg)
	r.mu.Lock()
	if stop, ok := r.heartbeats[id]; ok {
		close(stop)
		delete(r.heartbeats, id)
	}
	r.mu.Unlock()
	if err := r.client.Agent().ServiceDeregister(id); err != nil {
		return errutil.Explain(err, "registry-consul: deregister %q", reg.ServiceName)
	}
	return nil
}
