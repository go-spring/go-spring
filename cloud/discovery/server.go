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

package discovery

import (
	"context"

	"go-spring.org/log"
	"go-spring.org/spring/gs"
	"go-spring.org/stdlib/errutil"
)

// The registration core: the ONE server that owns this process's publication
// lifecycle across every configured discovery center. The backend starters
// (starter-discovery-etcd, starter-discovery-consul, starter-discovery-nacos,
// starter-discovery-zookeeper) each contribute one [Registry] per configured
// ${spring.discovery.<backend>.<name>} block; [Server] collects them all and
// drives them in lockstep — registered everywhere once the app is ready,
// deregistered everywhere as shutdown begins, weight changes broadcast to all.
// A backend missing from one center would split consumers' views, so any
// registration failure fails startup.
//
// It is a global / infrastructure-archetype component: it opens no port. Its
// bean is a [gs.Server], so registration plugs into the server lifecycle — the
// instance is published once the application is ready and deregistered as
// shutdown begins (via PreStop), so discovery stops handing it out before it
// actually stops serving. That ordering is what makes a rolling restart
// lossless.

var (
	// starterTag identifies logs emitted by the discovery core.
	starterTag = log.RegisterAppTag("discovery", "")
)

// Server publishes this instance to every configured discovery center as part
// of the Go-Spring server lifecycle. Its exported fields are populated by the
// container.
type Server struct {
	// Config is the instance identity, bound from ${spring.discovery} and
	// shared by every center.
	Config RegistrationConfig `value:"${spring.discovery}"`

	// Registries collects every backend's discovery bean — one per configured
	// discovery-center block, across all backends.
	Registries []Registry `autowire:"?"`

	inst Instance
}

// NewServer builds the registration server.
func NewServer() *Server { return &Server{} }

// Run publishes this instance to every registry once the application is
// ready, then blocks until shutdown. It validates the identity and that at
// least one discovery center is configured before signalling readiness, so a
// misconfiguration fails startup rather than surfacing later.
func (s *Server) Run(ctx context.Context, sig gs.ReadySignal) error {
	if s.Config.ServiceName == "" || s.Config.Addr == "" {
		return errutil.Explain(nil, "discovery: ${spring.discovery.service-name} and ${spring.discovery.addr} are required")
	}
	if len(s.Registries) == 0 {
		return errutil.Explain(nil, "discovery: ${spring.discovery.service-name} is set but no discovery center is configured (e.g. ${spring.discovery.etcd.<name>.endpoints})")
	}
	s.inst = Instance{
		ServiceName: s.Config.ServiceName,
		ID:          s.Config.ID,
		Addr:        s.Config.Addr,
		Weight:      s.Config.Weight,
		Version:     s.Config.Version,
		Zone:        s.Config.Zone,
		Scheme:      s.Config.Scheme,
		Metadata:    s.Config.Metadata,
	}

	<-sig.TriggerAndWait()

	// This instance's identity rides on the context from here on: the lines
	// below carry it without repeating it, as does every registry backend the
	// registration fans out into.
	ctx = log.WithFields(ctx, instanceFields(s.inst)...)

	log.Debug(ctx, starterTag, func() []log.Field {
		return []log.Field{
			log.Int("centers", len(s.Registries)),
			log.Msg("register the instance into the configured discovery centers"),
		}
	})
	for _, r := range s.Registries {
		if err := r.Register(ctx, s.inst); err != nil {
			log.Errorf(ctx, starterTag, err, "register service failed")
			return errutil.Explain(err, "discovery: register %q", s.inst.ServiceName)
		}
	}
	log.Info(ctx, starterTag,
		log.Int("centers", len(s.Registries)),
		log.Msg("register instance success"))

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
		return errutil.Explain(nil, "discovery: instance not registered yet")
	}
	ctx = log.WithFields(ctx, instanceFields(s.inst)...)
	for _, r := range s.Registries {
		if err := r.UpdateWeight(ctx, s.inst, weight); err != nil {
			log.Error(ctx, starterTag, err,
				log.Int("weight", weight),
				log.Msg("update weight failed"))
			return errutil.Explain(err, "discovery: update weight to %d", weight)
		}
	}
	return nil
}

func (s *Server) deregister(ctx context.Context) {
	ctx = log.WithFields(ctx, instanceFields(s.inst)...)
	for _, r := range s.Registries {
		if err := r.Deregister(ctx, s.inst); err != nil {
			log.Warn(ctx, starterTag,
				log.Err(err),
				log.Msg("deregister failed"))
		}
	}
}

// instanceFields returns the fields identifying the instance being
// registered. They are attached to a context with log.WithFields rather than
// repeated at each call site, and "service" is the name the metric and span
// instruments use.
func instanceFields(inst Instance) []log.Field {
	return []log.Field{
		log.String("service", inst.ServiceName),
		log.String("id", inst.ID),
		log.String("addr", inst.Addr),
	}
}

// RegistrationConfig binds the instance identity under ${spring.discovery}.
// These fields describe the instance itself and are shared by every registry
// backend this process registers into: configuring multiple centers
// (spring.discovery.etcd.*, spring.discovery.zookeeper.*, ...) fans ONE
// publication out to all of them — same service name, same address, same
// weight everywhere.
type RegistrationConfig struct {
	// ServiceName is the service this instance belongs to. Its presence is the
	// registration intent signal: set means this process publishes itself into
	// every configured discovery center, unset means pure consumer.
	ServiceName string `value:"${service-name:=}"`

	// ID identifies this instance within the service; empty derives one.
	ID string `value:"${id:=}"`

	// Addr is the instance's host:port as consumers dial it.
	Addr string `value:"${addr:=}"`

	// Weight is the initial load-balancing weight; 0 means drained.
	Weight int `value:"${weight:=100}"`

	// Version is the application version of this instance; empty means not
	// advertised. Consumers may route on it (e.g. canary by version).
	Version string `value:"${version:=}"`

	// Zone is the availability zone / unit this instance sits in; empty means
	// not advertised. Consumers may prefer same-zone instances.
	Zone string `value:"${zone:=}"`

	// Scheme selects the transport consumers should dial: "" or "tcp" for a
	// plain TCP connection (the default), "tls"/"https" when it requires TLS,
	// "http"/"https" for HTTP-level routing. It is advisory.
	Scheme string `value:"${scheme:=}"`

	// Metadata carries additional backend-agnostic attributes. Version, Zone
	// and Scheme are advertised separately and take precedence over same-key
	// entries here.
	Metadata map[string]string `value:"${metadata:=}"`
}
