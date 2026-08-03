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

// Package StarterRegistryConsul registers the current instance into a Consul
// service registry - the provider-side counterpart to client-side discovery.
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

var (
	// starterTag identifies logs emitted by the consul registry starter.
	starterTag = log.RegisterInfraTag("starter_registry_consul", "")
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
	).Name("registryServer").
		Export(gs.As[gs.Server]()).
		Condition(gs.OnProperty("spring.registry.consul.address"))
}

// NewServer builds the Consul registrar from c and returns the Server that
// publishes this instance on ready and deregisters on shutdown. Consul's agent
// client does not dial eagerly, so a bad address surfaces on the first Register
// rather than here.
func NewServer(c ConsulConfig) (*Server, error) {
	log.Debugf(context.Background(), starterTag, "creating consul registrar address=%s ttl=%s", c.Address, c.TTL)
	reg, err := newConsulRegistrar(c)
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

	log.Debugf(ctx, starterTag, "registering service=%s id=%s addr=%s weight=%d", s.reg.ServiceName, s.reg.ID, s.reg.Addr, s.reg.Weight)
	if err := s.registrar.Register(ctx, s.reg); err != nil {
		log.Errorf(ctx, starterTag, "register service=%s failed: %v", s.reg.ServiceName, err)
		return errutil.Explain(err, "registry: register %q", s.reg.ServiceName)
	}
	log.Infof(ctx, starterTag, "registered %q at %s", s.reg.ServiceName, s.reg.Addr)

	<-ctx.Done()
	return nil
}

// PreStop deregisters the instance as soon as shutdown begins - before the
// pre-stop delay and before any server stops - so discovery removes it while
// in-flight requests keep being served (the lossless-drain sequence).
func (s *Server) PreStop(ctx context.Context) {
	s.deregister(ctx)
}

// Stop deregisters as a fallback should PreStop not have run. Deregister is
// idempotent, so a second call is a no-op.
func (s *Server) Stop() error {
	s.deregister(context.Background())
	return nil
}

func (s *Server) deregister(ctx context.Context) {
	if s.registrar == nil {
		return
	}
	if err := s.registrar.Deregister(ctx, s.reg); err != nil {
		log.Warnf(ctx, starterTag, "deregister %q: %v", s.reg.ServiceName, err)
	}
}
