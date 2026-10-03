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

// Package StarterActuator exposes operational HTTP endpoints — health probes,
// build info, and runtime introspection — on a dedicated management port.
//
// The actuator is a plain gs.Server: it waits for the readiness barrier (all
// servers ready) before serving. During boot the startupProbe burns its budget
// on connection-refused retries — by design. The full working model (probe
// division of labor, mandatory startupProbe budget) lives in the starter's
// README; it is not restated here.
//
// Probe endpoints map to the three Kubernetes container probes. The z-suffixed
// paths are canonical; the older names are kept as aliases:
//
//	/healthz  (alias /health)    liveness: 200 {"status":"UP"} while serving;
//	          503 OUT_OF_SERVICE during graceful drain. Consults only
//	          indicators that declare the liveness group (usually none), so a
//	          degraded dependency never trips a liveness restart.
//	/readyz   (alias /readiness) readiness: 200 while every readiness-group
//	          indicator passes. When only non-critical indicators fail the
//	          status is DEGRADED with 200 (still serving; failures visible
//	          per-component). During graceful shutdown it flips to 503
//	          OUT_OF_SERVICE (see PreStop) so Kubernetes drains the pod before
//	          servers stop.
//	/startupz (alias /startup)   startup: the aggregate of every startup-group
//	          indicator. Backs the K8s startupProbe (see the working model
//	          above); the not-ready window itself is connection refused, not a
//	          503 from here. Unaffected by drain (startupProbe stops after
//	          first success).
//
// Introspection endpoints (all JSON unless noted):
//
//	GET  /info        build/version info from the embedded build metadata.
//
// Contributed endpoints such as /metrics register unconditionally: a
// component that contributes an endpoint means to expose it, and whether it
// exists at all is the contributor's own enable switch, not an actuator-side
// filter. Access control is the management port's business (see the Guard
// below), not per-endpoint whitelisting.
//
// The whole management port can be authenticated with
// spring.actuator.token (bearer) or spring.actuator.username /
// spring.actuator.password (HTTP Basic) via the security.Guard.
// When no scheme is configured and the listener binds non-loopback, a WARN is
// logged at startup.
//
// Health indicators are contributed by other beans: any bean exported as
// health.Indicator (a redis client wrapper, a gorm pool wrapper, ...) is
// collected here and folded into the probes with zero per-component wiring.
package StarterActuator

import (
	"context"
	"net"
	"net/http"
	"runtime/debug"
	"sync/atomic"

	"go-spring.org/cloud/actuator/endpoint"
	"go-spring.org/cloud/actuator/health"
	"go-spring.org/cloud/security"
	"go-spring.org/log"
	"go-spring.org/spring/gs"
	"go-spring.org/stdlib/errutil"
	"go-spring.org/stdlib/httputil"
	"go-spring.org/stdlib/netutil"
)

// Server serves the actuator endpoints on a dedicated management port.
type Server struct {
	// cfg is the actuator configuration, bound from ${spring.actuator}.
	cfg Config

	// indicators are all beans exported as health.Indicator. Optional: an app
	// with no indicators still gets liveness/readiness/info.
	indicators []*health.Indicator

	// endpoints are all beans exported as endpoint.Endpoint. Optional: a
	// component (e.g. starter-otel's Prometheus /metrics) contributes its handler
	// here and it is mounted on this same management port, so operators scrape
	// one port instead of each component running its own server. The actuator
	// does not import those components — the seam is the stdlib interface.
	endpoints []*endpoint.Endpoint

	svr      *http.Server
	draining atomic.Bool
}

// NewServer assembles the actuator from its configuration and the beans the
// container collected for it: the config is bound from ${spring.actuator}
// (gs.TagArg at the Provide site), and the indicator/endpoint collections are
// optional — an app with none registered still gets liveness/readiness/info.
func NewServer(cfg Config, indicators []*health.Indicator, endpoints []*endpoint.Endpoint) *Server {
	return &Server{cfg: cfg, indicators: indicators, endpoints: endpoints}
}

// Run binds the management listener, waits for the readiness barrier like any
// other gs.Server, then serves. See the README's working-model section for why
// the startup window intentionally answers nobody.
func (s *Server) Run(ctx context.Context, sig gs.ReadySignal) error {
	ln, err := net.Listen("tcp", s.cfg.Address)
	if err != nil {
		log.Errorf(ctx, log.TagAppDef, "failed to listen on %s: %v", s.cfg.Address, err)
		return errutil.Explain(err, "actuator: failed to listen on %s", s.cfg.Address)
	}

	log.Infof(ctx, log.TagAppDef, "actuator listening on %s", s.cfg.Address)

	s.svr = &http.Server{
		Handler:           s.buildHandler(ctx),
		ReadHeaderTimeout: s.cfg.ReadHeaderTimeout,
		ReadTimeout:       s.cfg.ReadTimeout,
		WriteTimeout:      s.cfg.WriteTimeout,
		IdleTimeout:       s.cfg.IdleTimeout,
	}

	// Signal this server ready (the listener is bound and the handler built —
	// there is nothing else to prepare) and wait for every other server: only
	// then does serving begin.
	<-sig.TriggerAndWait()

	err = s.svr.Serve(ln)
	if errutil.IsServerClosed(err) {
		log.Infof(ctx, log.TagAppDef, "actuator server closed gracefully")
		return nil
	}
	log.Errorf(ctx, log.TagAppDef, "actuator serve error: %v", err)
	return errutil.Explain(err, "actuator: failed to serve on %s", s.cfg.Address)
}

// buildHandler assembles the management port's handler: probe endpoints,
// introspection endpoints, and contributed endpoints (e.g. otel's Prometheus
// /metrics) all register unconditionally, wrapped with the
// authentication guard configured via spring.actuator.token or
// spring.actuator.username/.password. When no scheme is configured and the
// listener is reachable off-host, a WARN is logged — the same posture the
// pprof starter takes.
func (s *Server) buildHandler(ctx context.Context) http.Handler {
	mux := http.NewServeMux()

	// Probe endpoints — K8s convention.
	mux.HandleFunc("GET /healthz", s.handleLiveness)
	mux.HandleFunc("GET /readyz", s.handleReadiness)
	mux.HandleFunc("GET /startupz", s.handleStartup)

	// Probe endpoints — Spring Boot Actuator convention.
	mux.HandleFunc("GET /health", s.handleLiveness)
	mux.HandleFunc("GET /readiness", s.handleReadiness)
	mux.HandleFunc("GET /startup", s.handleStartup)

	// Introspection endpoints (built-in) and contributed endpoints (e.g.
	// otel's Prometheus /metrics) share one registration path — they are one
	// model. Built-ins come first so a contributor cannot shadow /health etc.
	// (ServeMux panics on a duplicate pattern, surfacing a misconfiguration
	// at startup).
	// mounted lists every endpoint mounted on this port: the built-in /info
	// first, then the contributed ones.
	endpoints := append([]*endpoint.Endpoint{
		{Pattern: "GET /info", Handler: http.HandlerFunc(s.handleInfo)},
	}, s.endpoints...)
	for _, e := range endpoints {
		mux.Handle(e.Pattern, e.Handler)
		log.Debugf(ctx, log.TagAppDef, "registered endpoint: %s", e.Pattern)
	}

	guard := security.Guard{Token: s.cfg.Token, Username: s.cfg.Username, Password: s.cfg.Password}
	if !guard.Enabled() && !netutil.IsLoopback(s.cfg.Address) {
		log.Warnf(ctx, log.TagAppDef,
			"actuator listening on %q without authentication; set ${spring.actuator.token} or ${spring.actuator.username}/${spring.actuator.password}",
			s.cfg.Address)
	}
	return guard.Wrap(mux)
}

// Stop gracefully shuts down the management server, propagating ctx into
// http.Server.Shutdown so the drain rides the shutdown context.
func (s *Server) Stop(ctx context.Context) error {
	if s.svr == nil {
		return nil
	}
	log.Infof(ctx, log.TagAppDef, "stopping actuator server")
	return s.svr.Shutdown(ctx)
}

// PreStop implements the framework's graceful-drain hook. It is called at the
// start of shutdown, before the server is stopped, and flips readiness to
// OUT_OF_SERVICE so a Kubernetes readiness probe fails and the endpoint
// controller removes this pod from Service endpoints while in-flight requests
// keep being served. The management server itself keeps serving so probes can
// still observe the OUT_OF_SERVICE state during the drain window.
func (s *Server) PreStop(ctx context.Context) {
	s.draining.Store(true)
}

// handleInfo reports build/version metadata read from the binary's embedded
// build info (module path/version, Go toolchain, target platform, VCS stamp
// when the binary was built from a checkout, and whether paths were trimmed).
// Everything here is static per binary — runtime state belongs to /metrics.
func (s *Server) handleInfo(w http.ResponseWriter, r *http.Request) {
	info := map[string]any{}
	if bi, ok := debug.ReadBuildInfo(); ok {
		info["go"] = bi.GoVersion
		info["module"] = map[string]string{
			"path":    bi.Main.Path,
			"version": bi.Main.Version,
		}
		vcs := map[string]string{}
		for _, setting := range bi.Settings {
			switch setting.Key {
			case "vcs.revision", "vcs.time", "vcs.modified":
				vcs[setting.Key] = setting.Value
			case "GOOS":
				info["os"] = setting.Value
			case "GOARCH":
				info["arch"] = setting.Value
			case "-trimpath":
				info["trimpath"] = setting.Value == "true"
			}
		}
		if len(vcs) > 0 {
			info["vcs"] = vcs
		}
	}
	httputil.WriteJSON(w, http.StatusOK, info)
}
