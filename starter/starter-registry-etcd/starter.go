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

// Package StarterRegistryEtcd registers the current instance into an etcd
// cluster — the provider-side counterpart to client-side discovery.
//
// It exists for VM / bare-metal / hybrid deployments where the platform does
// not register instances for you. In pure Kubernetes the platform already
// registers every Pod behind a Service, so you would use starter-discovery-k8s
// to *discover* peers and not register at all. RPC-framework provider
// registration is out of scope and stays framework-native (starter/DESIGN §3);
// this starter publishes a plain instance (any transport) to etcd.
//
// Each instance is written under a key bound to its own lease and kept alive by
// a background keep-alive. If the process dies without deregistering, the lease
// expires and etcd deletes the key — self-healing without a reaper.
//
// This is a global / infrastructure-archetype starter (starter/DESIGN §2.4): it
// opens no port. It exports a gs.Server so registration plugs into the server
// lifecycle — the instance is published once the application is ready and
// deregistered as shutdown begins (via PreStop), so discovery stops handing it
// out before it actually stops serving. That ordering is what makes a rolling
// restart lossless.
//
// Blank-import the package and configure it:
//
//	spring.registry.etcd.endpoints=127.0.0.1:2379
//	spring.registry.service-name=orders
//	spring.registry.addr=10.0.0.5:8080
package StarterRegistryEtcd

import (
	"context"
	"runtime"

	"go-spring.org/log"
	"go-spring.org/spring/conf"
	"go-spring.org/spring/gs"
	"go-spring.org/spring/discovery"
	"go-spring.org/stdlib/errutil"
	"go-spring.org/stdlib/flatten"
)

func init() {
	// Activated only when etcd endpoints are set. The module callback runs in
	// the bean-registration phase — before any Server.Run — so the registrar is
	// in the stdlib/discovery registry by the time the register server looks it
	// up. Registering the backend here (not as a bean) mirrors starter-discovery-k8s.
	_, file, line, _ := runtime.Caller(0)
	gs.Module(gs.OnProperty("spring.registry.etcd.endpoints"), func(r gs.BeanProvider, p flatten.Storage) error {
		var ec EtcdConfig
		if err := conf.Bind(p, &ec, "${spring.registry.etcd}"); err != nil {
			return err
		}
		if _, ok := discovery.GetRegistrar(ec.Name); ok {
			return errutil.Explain(nil, "registry-etcd: registrar %q already registered", ec.Name)
		}
		reg, err := newEtcdRegistrar(ec)
		if err != nil {
			return errutil.Explain(err, "registry-etcd: build registrar %q", ec.Name)
		}
		discovery.RegisterRegistrar(ec.Name, reg)

		bean := r.Provide(func() *Server { return &Server{} }).
			Name("registryServer").
			Export(gs.As[gs.Server]())
		bean.SetFileLine(file, line)
		return nil
	})
}

// Server publishes this instance to a service registry as part of the Go-Spring
// server lifecycle. It opens no network port; its only job is Register-on-ready
// and Deregister-on-shutdown. Its exported field is populated by the container.
type Server struct {
	// Config is bound from ${spring.registry}.
	Config RegistrationConfig `value:"${spring.registry}"`

	registrar discovery.Registrar
	reg       discovery.Registration
}

// Run resolves the configured backend and, once the application is ready,
// publishes this instance, then blocks until shutdown. It validates required
// fields before signalling readiness so a misconfiguration fails startup rather
// than surfacing later.
func (s *Server) Run(ctx context.Context, sig gs.ReadySignal) error {
	if s.Config.ServiceName == "" || s.Config.Addr == "" {
		return errutil.Explain(nil, "registry: ${spring.registry.service-name} and ${spring.registry.addr} are required")
	}
	r, err := discovery.MustGetRegistrar(s.Config.Backend)
	if err != nil {
		return err
	}
	s.registrar = r
	s.reg = discovery.Registration{
		ServiceName: s.Config.ServiceName,
		ID:          s.Config.ID,
		Addr:        s.Config.Addr,
		Weight:      s.Config.Weight,
		Metadata:    s.Config.Metadata,
	}

	<-sig.TriggerAndWait()

	if err := s.registrar.Register(ctx, s.reg); err != nil {
		return errutil.Explain(err, "registry: register %q", s.reg.ServiceName)
	}
	log.Infof(ctx, log.TagAppDef, "registry: registered %q at %s (backend %q)",
		s.reg.ServiceName, s.reg.Addr, s.Config.Backend)

	<-ctx.Done()
	return nil
}

// PreStop deregisters the instance as soon as shutdown begins — before the
// pre-stop delay and before any server stops — so discovery removes it while
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
		log.Warnf(ctx, log.TagAppDef, "registry: deregister %q: %v", s.reg.ServiceName, err)
	}
}
