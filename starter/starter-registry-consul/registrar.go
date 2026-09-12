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
	"errors"
	"net"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/hashicorp/consul/api"
	"go-spring.org/cloud/discovery"
	"go-spring.org/log"
	"go-spring.org/stdlib/errutil"
)

// instance is the process advertisement the registrar publishes; it is the
// neutral [discovery.Instance] contract. The crash-safety contract every
// registry starter follows lives in starter/DESIGN §3 (Register must
// self-renew so correctness never depends on Deregister).
type instance = discovery.Instance

// consulRegistrar publishes instances to a Consul agent and keeps each one live
// by passing its TTL health check on a background heartbeat until Deregister.
type consulRegistrar struct {
	client                  *api.Client
	ttl                     time.Duration
	deregisterCriticalAfter time.Duration

	mu         sync.Mutex
	heartbeats map[string]chan struct{} // service ID -> heartbeat stop channel
	regs       map[string]instance      // service ID -> last registered value
}

// newConsulRegistrar returns a registrar writing through client (the shared
// center client; the agent was already probed when client was built) with c's
// TTL and auto-deregistration window. It does NOT close client — the owner
// (consulCenter, or the standalone fallback in starter.go) does.
func newConsulRegistrar(c ConsulConfig, client *api.Client) (*consulRegistrar, error) {
	if client == nil {
		return nil, errutil.Explain(nil, "registry-consul: nil consul client")
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

// normalizeWeight clamps a misconfigured negative weight to 1 at write time.
// 0 passes through as the drain signal; an unset weight is expressed by the
// ${spring.registry.weight} default, not by this clamp. Register and
// UpdateWeight share it so both write paths agree on the contract.
func normalizeWeight(w int) int {
	if w < 0 {
		return 1
	}
	return w
}

// Register publishes reg with a TTL health check, passes the check immediately
// so the instance is healthy without waiting a full TTL, then keeps it passing
// on a background heartbeat until Deregister.
func (r *consulRegistrar) Register(ctx context.Context, reg instance) error {
	reg.Weight = normalizeWeight(reg.Weight)
	if _, _, err := net.SplitHostPort(reg.Addr); err != nil {
		return errutil.Explain(err, "registry-consul: addr %q must be host:port", reg.Addr)
	}
	if _, err := strconv.Atoi(reg.Addr[strings.LastIndex(reg.Addr, ":")+1:]); err != nil {
		return errutil.Explain(err, "registry-consul: addr %q has a non-numeric port", reg.Addr)
	}
	id := serviceID(reg)
	checkID := "service:" + id
	attempt := discovery.RegisterAttempt(ctx, obsSystem, reg.ServiceName, discovery.ReasonInitial)
	err := r.upsert(reg)
	attempt(err)
	if err != nil {
		return errutil.Explain(err, "registry-consul: register %q", reg.ServiceName)
	}
	// The first TTL pass is best-effort: the heartbeat below retries it every
	// half TTL, so a failure here only delays "passing", it never fails Register.
	if err := r.client.Agent().UpdateTTL(checkID, "", api.HealthPassing); err != nil {
		log.Warnf(ctx, starterTag, "consul initial TTL pass for check=%s failed: %v", checkID, err)
	}

	stop := make(chan struct{})
	r.mu.Lock()
	// Re-registering the same instance refreshes it: retire the old heartbeat.
	if old, ok := r.heartbeats[id]; ok {
		close(old)
	}
	r.heartbeats[id] = stop
	r.regs[id] = reg
	r.mu.Unlock()

	go r.heartbeat(id, stop)
	return nil
}

// reRegister re-runs the (idempotent) service upsert for id. It is the
// self-healing escalation of the heartbeat: if Consul dropped the service —
// the check went critical past DeregisterCriticalServiceAfter, or the local
// agent restarted and lost it — UpdateTTL alone can never recover (the check
// is gone), while ServiceRegister recreates service and check, after which the
// regular TTL passes keep it alive again.
func (r *consulRegistrar) reRegister(id string) error {
	r.mu.Lock()
	reg, ok := r.regs[id]
	r.mu.Unlock()
	if !ok {
		return nil
	}
	report := discovery.RegisterAttempt(context.Background(), obsSystem, reg.ServiceName, discovery.ReasonSelfHeal)
	err := r.upsert(reg)
	report(err)
	return err
}

// upsert writes reg's full service-and-check registration, weight included. It
// is the single write step shared by Register, the heartbeat's self-healing
// re-register, and UpdateWeight, so all three advertise the identical entry
// apart from the weight.
func (r *consulRegistrar) upsert(reg instance) error {
	return r.client.Agent().ServiceRegister(r.buildRegistration(reg))
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
func (r *consulRegistrar) UpdateWeight(ctx context.Context, reg instance, weight int) error {
	id := serviceID(reg)
	r.mu.Lock()
	last, ok := r.regs[id]
	r.mu.Unlock()
	if !ok {
		return errutil.Explain(nil, "registry-consul: update weight for unregistered instance %q", id)
	}
	last.Weight = normalizeWeight(weight)
	report := discovery.WeightChange(ctx, obsSystem, last.ServiceName)
	err := r.upsert(last)
	report(err)
	if err != nil {
		return errutil.Explain(err, "registry-consul: update weight %q", last.ServiceName)
	}
	r.mu.Lock()
	r.regs[id] = last
	r.mu.Unlock()
	return nil
}

// heartbeat re-passes the TTL check of service id at half the TTL until stop
// is closed. A failed pass can never be "just dropped": failures are logged
// (escalating to Error once they persist), and once they do persist the
// service is re-registered — an idempotent upsert — because if the outage
// outlasted DeregisterCriticalServiceAfter (or the agent restarted), the
// check no longer exists and only a re-register brings the instance back.
func (r *consulRegistrar) heartbeat(id string, stop <-chan struct{}) {
	checkID := "service:" + id
	interval := r.ttl / 2
	if interval <= 0 {
		interval = r.ttl
	}
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	const persistAfter = 3
	var failures int
	for {
		select {
		case <-stop:
			return
		case <-ticker.C:
			err := r.client.Agent().UpdateTTL(checkID, "", api.HealthPassing)
			if err == nil {
				failures = 0
				continue
			}
			failures++
			if failures >= persistAfter {
				log.Errorf(context.Background(), starterTag,
					"consul TTL heartbeat for check=%s failed %d times in a row; re-registering the service to recover: %v",
					checkID, failures, err)
				// Re-register (upsert) instead of only logging: recreates the
				// service and check if Consul already dropped them.
				if rerr := r.reRegister(id); rerr != nil {
					log.Errorf(context.Background(), starterTag,
						"consul re-register for service=%s failed: %v", id, rerr)
				}
			} else {
				log.Warnf(context.Background(), starterTag,
					"consul TTL heartbeat for check=%s failed (%d/%d): %v", checkID, failures, persistAfter, err)
			}
		}
	}
}

// Deregister stops the heartbeat and removes the instance. It is idempotent:
// deregistering an instance that is not registered is a no-op that still asks
// Consul to drop the id, and Consul answering 404 for an id it does not hold is
// that no-op — the registry core calls Deregister from both PreStop and the
// Stop fallback, so a repeat call is the normal shutdown path, not a failure.
func (r *consulRegistrar) Deregister(ctx context.Context, reg instance) error {
	id := serviceID(reg)
	r.mu.Lock()
	if stop, ok := r.heartbeats[id]; ok {
		close(stop)
		delete(r.heartbeats, id)
	}
	delete(r.regs, id)
	r.mu.Unlock()
	report := discovery.DeregisterAttempt(ctx, obsSystem, reg.ServiceName)
	err := r.client.Agent().ServiceDeregister(id)
	var statusErr api.StatusError
	if errors.As(err, &statusErr) && statusErr.Code == http.StatusNotFound {
		err = nil // already gone: the desired end state holds
	}
	report(err)
	if err != nil {
		return errutil.Explain(err, "registry-consul: deregister %q", reg.ServiceName)
	}
	return nil
}
