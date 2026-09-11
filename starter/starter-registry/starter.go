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

// Package StarterRegistry is the registration core: the ONE server that owns
// this process's publication lifecycle across every configured registry
// center. The backend starters (starter-registry-etcd, starter-registry-consul,
// starter-registry-nacos, starter-registry-zookeeper) each contribute one
// discovery.Registrar per configured ${spring.registry.<backend>.<name>}
// block; this server collects them all and drives them in lockstep —
// registered everywhere once the app is ready, deregistered everywhere as
// shutdown begins, weight changes broadcast to all. A backend missing from
// one center would split consumers' views, so any registration failure fails
// startup.
//
// Importing any backend starter imports this package transitively (they all
// depend on it), so the server exists exactly once per process by Go's
// package-init semantics — no coordination between backends is needed.
//
// It is a global / infrastructure-archetype starter: it opens no port. It
// exports a gs.Server so registration plugs into the server lifecycle — the
// instance is published once the application is ready and deregistered as
// shutdown begins (via PreStop), so discovery stops handing it out before it
// actually stops serving. That ordering is what makes a rolling restart
// lossless.
package StarterRegistry

import (
	"context"

	"go-spring.org/cloud/discovery"
	"go-spring.org/log"
	"go-spring.org/spring/gs"
	"go-spring.org/stdlib/errutil"
)

var (
	// starterTag identifies logs emitted by the registry core.
	starterTag = log.RegisterAppTag("registry", "")
)

func init() {
	// Activated only when the registration intent signal is set: a
	// ${spring.registry.service-name} means this process publishes itself.
	// Pure consumers leave it unset and nothing registers anywhere. The
	// Registrars field is the container's slice collection of every
	// discovery.Registrar bean the backend starters derived from their
	// configured blocks.
	gs.Provide(NewServer).
		Name("registryServer").
		Export(gs.As[gs.Server]()).
		Condition(gs.OnProperty("spring.registry.service-name"))
}

// Server publishes this instance to every configured registry center as part
// of the Go-Spring server lifecycle. Its exported fields are populated by the
// container.
type Server struct {
	// Config is the instance identity, bound from ${spring.registry} and
	// shared by every center.
	Config RegistrationConfig `value:"${spring.registry}"`

	// Registrars collects every backend's registrar bean — one per configured
	// registry-center block, across all backends.
	Registrars []discovery.Registrar `autowire:"?"`

	inst discovery.Instance
}

// NewServer builds the registration server.
func NewServer() *Server { return &Server{} }

// Run publishes this instance to every registrar once the application is
// ready, then blocks until shutdown. It validates the identity and that at
// least one registry center is configured before signalling readiness, so a
// misconfiguration fails startup rather than surfacing later.
func (s *Server) Run(ctx context.Context, sig gs.ReadySignal) error {
	if s.Config.ServiceName == "" || s.Config.Addr == "" {
		return errutil.Explain(nil, "registry: ${spring.registry.service-name} and ${spring.registry.addr} are required")
	}
	if len(s.Registrars) == 0 {
		return errutil.Explain(nil, "registry: ${spring.registry.service-name} is set but no registry center is configured (e.g. ${spring.registry.etcd.<name>.endpoints})")
	}
	s.inst = discovery.Instance{
		ServiceName: s.Config.ServiceName,
		ID:          s.Config.ID,
		Addr:        s.Config.Addr,
		Weight:      s.Config.Weight,
		Metadata:    s.Config.Metadata,
	}

	<-sig.TriggerAndWait()

	log.Debugf(ctx, starterTag, "registering service=%s id=%s addr=%s weight=%d into %d registry center(s)", s.inst.ServiceName, s.inst.ID, s.inst.Addr, s.inst.Weight, len(s.Registrars))
	for _, r := range s.Registrars {
		if err := r.Register(ctx, s.inst); err != nil {
			log.Errorf(ctx, starterTag, "register service=%s failed: %v", s.inst.ServiceName, err)
			return errutil.Explain(err, "registry: register %q", s.inst.ServiceName)
		}
	}
	log.Infof(ctx, starterTag, "registered %q at %s in %d registry center(s)", s.inst.ServiceName, s.inst.Addr, len(s.Registrars))

	<-ctx.Done()
	return nil
}

// PreStop deregisters the instance from every center as soon as shutdown
// begins - before the pre-stop delay and before any server stops - so
// discovery removes it while in-flight requests keep being served (the
// lossless-drain sequence).
func (s *Server) PreStop(ctx context.Context) {
	s.deregister(ctx)
}

// Stop deregisters as a fallback should PreStop not have run, propagating ctx
// into each Deregister call. Deregister is idempotent, so repeat calls are
// no-ops.
func (s *Server) Stop(ctx context.Context) error {
	s.deregister(ctx)
	return nil
}

// UpdateWeight re-advertises this instance with a new weight in every center
// on its existing registration: the entry never leaves discovery and watchers
// (loadbalance pools included) simply observe the new value on their next
// snapshot. It is an optional runtime API — call it after Run has registered
// the instance.
func (s *Server) UpdateWeight(ctx context.Context, weight int) error {
	if s.inst.Addr == "" {
		return errutil.Explain(nil, "registry: instance not registered yet")
	}
	for _, r := range s.Registrars {
		if err := r.UpdateWeight(ctx, s.inst, weight); err != nil {
			return errutil.Explain(err, "registry: update weight to %d", weight)
		}
	}
	return nil
}

func (s *Server) deregister(ctx context.Context) {
	for _, r := range s.Registrars {
		if err := r.Deregister(ctx, s.inst); err != nil {
			log.Warnf(ctx, starterTag, "deregister %q: %v", s.inst.ServiceName, err)
		}
	}
}
