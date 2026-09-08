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

// Package StarterRegistryConsul adapts Consul as the service registry, serving
// BOTH sides of the naming idiom: the provider-side write (register /
// deregister this instance, this file) and the client-side read (discovery
// backends, discovery_consul.go).
//
// It exists for VM / bare-metal / hybrid deployments where the platform does
// not register instances for you. In pure Kubernetes the platform already
// registers every Pod behind a Service, so you would use starter-discovery-k8s
// to *discover* peers and not register at all. RPC-framework provider
// registration is out of scope and stays framework-native (starter/DESIGN §3);
// this starter publishes a plain instance (any transport) to Consul.
//
// This is a global / infrastructure-archetype starter (starter/DESIGN §2.4): it
// opens no port. It exports a gs.Server so registration plugs into the server
// lifecycle - the instance is published once the application is ready and
// deregistered as shutdown begins (via PreStop), so discovery stops handing it
// out before it actually stops serving. That ordering is what makes a rolling
// restart lossless.
//
// Blank-import the package and configure it:
//
//	spring.registry.consul.address=127.0.0.1:8500
//	spring.registry.service-name=orders
//	spring.registry.addr=10.0.0.5:8080
package StarterRegistryConsul

import (
	"context"

	"go-spring.org/log"
	"go-spring.org/spring/gs"
	"go-spring.org/stdlib/errutil"
)

func init() {
	// Activated only when a Consul address is set. The constructor binds
	// ConsulConfig from ${spring.registry.consul} and builds the Consul
	// registrar. The instance to advertise is bound separately into Server.Config
	// from ${spring.registry}. The registrar is a local value held by the Server,
	// not a globally registered backend: this starter owns the full
	// register/deregister lifecycle.
	gs.Provide(
		NewServer,
		gs.TagArg("${spring.registry.consul}"),
		gs.TagArg("?"),
	).Name("registryServer").
		Export(gs.As[gs.Server]()).
		Condition(gs.OnProperty("spring.registry.consul.address"))
}

// NewServer builds the Consul registrar from c and returns the Server that
// publishes this instance on ready and deregisters on shutdown. cc is the
// shared center client bean for ${spring.registry.consul}; when it is nil (a
// direct construction, e.g. tests), a standalone probed client is built. The
// shared client is not closed here — the center bean owns it (and the Consul
// api client needs no closing anyway).
func NewServer(c ConsulConfig, cc *consulCenter) (*Server, error) {
	log.Debugf(context.Background(), log.TagAppDef, "creating consul registrar address=%s ttl=%s", c.Address, c.TTL)
	if cc == nil {
		var err error
		cc, err = newConsulCenter(c)
		if err != nil {
			return nil, err
		}
	}
	reg, err := newConsulRegistrar(c, cc.client)
	if err != nil {
		return nil, errutil.Explain(err, "registry-consul: build registrar")
	}
	return &Server{registrar: reg}, nil
}

// Server publishes this instance to a service registry as part of the Go-Spring
// server lifecycle. It opens no network port; its only job is Register-on-ready
// and Deregister-on-shutdown. Its exported field is populated by the container.
type Server struct {
	// Config is bound from ${spring.registry}.
	Config RegistrationConfig `value:"${spring.registry}"`

	registrar *consulRegistrar
	reg       instance
}

// Run publishes this instance once the application is ready, then blocks until
// shutdown. It validates required fields before signalling readiness so a
// misconfiguration fails startup rather than surfacing later.
func (s *Server) Run(ctx context.Context, sig gs.ReadySignal) error {
	if s.Config.ServiceName == "" || s.Config.Addr == "" {
		return errutil.Explain(nil, "registry: ${spring.registry.service-name} and ${spring.registry.addr} are required")
	}
	s.reg = instance{
		ServiceName: s.Config.ServiceName,
		ID:          s.Config.ID,
		Addr:        s.Config.Addr,
		Weight:      s.Config.Weight,
		Metadata:    s.Config.Metadata,
	}

	<-sig.TriggerAndWait()

	log.Debugf(ctx, log.TagAppDef, "registering service=%s id=%s addr=%s weight=%d", s.reg.ServiceName, s.reg.ID, s.reg.Addr, s.reg.Weight)
	if err := s.registrar.Register(ctx, s.reg); err != nil {
		log.Errorf(ctx, log.TagAppDef, "register service=%s failed: %v", s.reg.ServiceName, err)
		return errutil.Explain(err, "registry: register %q", s.reg.ServiceName)
	}
	log.Infof(ctx, log.TagAppDef, "registered %q at %s", s.reg.ServiceName, s.reg.Addr)

	<-ctx.Done()
	return nil
}

// PreStop deregisters the instance as soon as shutdown begins - before the
// pre-stop delay and before any server stops - so discovery removes it while
// in-flight requests keep being served (the lossless-drain sequence).
func (s *Server) PreStop(ctx context.Context) {
	s.deregister(ctx)
}

// Stop deregisters as a fallback should PreStop not have run,
// propagating the shutdown context into the deregister call. Deregister is
// idempotent, so a second call is a no-op.
func (s *Server) Stop(ctx context.Context) error {
	s.deregister(ctx)
	return nil
}

// UpdateWeight re-advertises this instance with a new weight: the entry never
// leaves discovery and watchers (loadbalance pools included) simply observe
// the new value on their next snapshot. A weight of 0 drains the instance —
// it receives no traffic without being deregistered. It is an optional
// runtime API — call it after Run has registered the instance.
func (s *Server) UpdateWeight(ctx context.Context, weight int) error {
	if s.registrar == nil || s.reg.Addr == "" {
		return errutil.Explain(nil, "registry-consul: instance not registered yet")
	}
	return s.registrar.UpdateWeight(ctx, s.reg, weight)
}

func (s *Server) deregister(ctx context.Context) {
	if s.registrar == nil {
		return
	}
	if err := s.registrar.Deregister(ctx, s.reg); err != nil {
		log.Warnf(ctx, log.TagAppDef, "deregister %q: %v", s.reg.ServiceName, err)
	}
}
