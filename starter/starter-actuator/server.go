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
// Unlike the application's main HTTP server (gs.SimpleHttpServer), which only
// begins serving once every server has signaled readiness, the actuator starts
// serving the moment its listener is bound. This is deliberate: a readiness
// probe must be able to reach the endpoint *before* the app is ready so it can
// observe the OUT_OF_SERVICE -> UP transition, and a liveness probe must answer
// throughout a long startup so the pod is not killed prematurely.
//
// Probe endpoints map to the three Kubernetes container probes. The z-suffixed
// paths are canonical; the older names are kept as aliases:
//
//	/healthz  (alias /health)    liveness: 200 {"status":"UP"} as long as the
//	          process is serving. Consults only indicators that declare the
//	          liveness group (usually none), so a degraded dependency never
//	          trips a liveness restart.
//	/readyz   (alias /readiness) readiness: 200 only after the app reports ready
//	          AND every readiness-group indicator passes; 503 otherwise. When
//	          only non-critical indicators fail the status is DEGRADED with 200
//	          (still serving; failures visible per-component). During graceful
//	          shutdown it flips to 503 OUT_OF_SERVICE (see PreStop) so Kubernetes
//	          drains the pod before servers stop.
//	/startupz (alias /startup)   startup: 503 until the app has finished starting
//	          AND every startup-group indicator passes, then 200. Backs a K8s
//	          startupProbe so a slow boot is not killed by the liveness probe.
//	          Unaffected by drain (startupProbe stops after first success).
//
// Introspection endpoints (all JSON unless noted):
//
//	GET  /info        build/version info from the embedded build metadata.
//
// /info is gated by spring.actuator.endpoints.include (non-empty include is
// a whitelist for it). Contributed endpoints such as /metrics stay
// default-on unless whitelisted; a contributed endpoint that declares
// itself sensitive (Endpoint.Sensitive) registers only when explicitly
// listed. The probe endpoints are always registered — filtering them would
// break the Kubernetes contract.
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
	"encoding/json"
	"net"
	"net/http"
	"runtime/debug"
	"strings"
	"sync/atomic"
	"time"

	"go-spring.org/cloud/actuator/endpoint"
	"go-spring.org/cloud/actuator/health"
	"go-spring.org/cloud/security"
	"go-spring.org/log"
	"go-spring.org/spring/gs"
	"go-spring.org/stdlib/errutil"
	"go-spring.org/stdlib/netutil"
)

// Server serves the actuator endpoints on a dedicated management port.
type Server struct {
	// Cfg is the actuator configuration, bound from ${spring.actuator}.
	Cfg Config

	// Indicators are all beans exported as health.Indicator. Optional: an app
	// with no indicators still gets liveness/readiness/info.
	Indicators []*health.Indicator

	// Endpoints are all beans exported as endpoint.Endpoint. Optional: a
	// component (e.g. starter-otel's Prometheus /metrics) contributes its handler
	// here and it is mounted on this same management port, so operators scrape
	// one port instead of each component running its own server. The actuator
	// does not import those components — the seam is the stdlib interface.
	Endpoints []*endpoint.Endpoint

	svr      *http.Server
	ready    atomic.Bool
	draining atomic.Bool
}

// NewServer assembles the actuator from its configuration and the beans the
// container collected for it: the config is bound from ${spring.actuator}
// (gs.TagArg at the Provide site), and the indicator/endpoint collections are
// optional — an app with none registered still gets liveness/readiness/info.
func NewServer(cfg Config, indicators []*health.Indicator, endpoints []*endpoint.Endpoint) *Server {
	return &Server{Cfg: cfg, Indicators: indicators, Endpoints: endpoints}
}

// Run binds the management listener and begins serving immediately. It
// contributes to the application readiness aggregate via sig, and flips its own
// readiness flag once every server (including this one) has reported ready.
func (s *Server) Run(ctx context.Context, sig gs.ReadySignal) error {
	ln, err := net.Listen("tcp", s.Cfg.Address)
	if err != nil {
		log.Errorf(ctx, log.TagAppDef, "failed to listen on %s: %v", s.Cfg.Address, err)
		return errutil.Explain(err, "actuator: failed to listen on %s", s.Cfg.Address)
	}

	log.Infof(ctx, log.TagAppDef, "actuator listening on %s", s.Cfg.Address)

	s.svr = &http.Server{
		Handler:           s.buildHandler(ctx),
		ReadHeaderTimeout: 5 * time.Second,
	}

	// Signal this server ready right away (so the app can proceed past its
	// readiness barrier) and watch the shared channel: it closes once all
	// servers are ready, at which point /readiness may return UP. We do NOT
	// block on it before serving — probes must reach us during startup.
	allReady := sig.TriggerAndWait()
	go func() {
		<-allReady
		s.ready.Store(true)
	}()

	err = s.svr.Serve(ln)
	if errutil.IsServerClosed(err) {
		log.Infof(ctx, log.TagAppDef, "actuator server closed gracefully")
		return nil
	}
	log.Errorf(ctx, log.TagAppDef, "actuator serve error: %v", err)
	return errutil.Explain(err, "actuator: failed to serve on %s", s.Cfg.Address)
}

// buildHandler assembles the management port's handler: probe endpoints
// (unconditional), introspection endpoints (through the include
// filter), contributed endpoints (same filter), all wrapped with the
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
	// otel's Prometheus /metrics) go through the same include filter and the
	// same registration path — they are one model. Built-ins come first so a
	// contributor cannot shadow /health etc. (ServeMux panics on a duplicate
	// pattern, surfacing a misconfiguration at startup). The built-in /info is
	// not sensitive (build metadata only); the probe endpoints above are
	// deliberately not in this table — they are always registered so the
	// Kubernetes contract cannot be broken by a filtering typo.
	eps := append([]*endpoint.Endpoint{
		{Pattern: "GET /info", Handler: http.HandlerFunc(s.handleInfo)},
	}, s.Endpoints...)
	for _, ep := range eps {
		if !s.endpointEnabled(ctx, ep) {
			continue
		}
		mux.Handle(ep.Pattern, ep.Handler)
		log.Debugf(ctx, log.TagAppDef, "registered endpoint: %s", ep.Pattern)
	}

	guard := security.Guard{Token: s.Cfg.Token, Username: s.Cfg.Username, Password: s.Cfg.Password}
	if !guard.Enabled() && !netutil.IsLoopback(s.Cfg.Address) {
		log.Warnf(ctx, log.TagAppDef,
			"actuator listening on %q without authentication; set ${spring.actuator.token} or ${spring.actuator.username}/${spring.actuator.password}",
			s.Cfg.Address)
	}
	return guard.Wrap(mux)
}

// endpointPath derives the include-filter key from a ServeMux pattern: the
// method prefix (if any) is dropped, so "GET /loggers" and "/metrics" both
// yield their path ("/loggers", "/metrics") — the endpoint's native identity.
func endpointPath(pattern string) string {
	if i := strings.IndexByte(pattern, ' '); i >= 0 {
		pattern = pattern[i+1:]
	}
	return pattern
}

// endpointEnabled reports whether ep passes the include filter. Sensitive
// endpoints require explicit inclusion even when include is empty
// (default-off). Otherwise, include non-empty means whitelist mode: everything
// not listed is off. Matching is exact and case-insensitive on the path.
func (s *Server) endpointEnabled(ctx context.Context, ep *endpoint.Endpoint) bool {
	path := endpointPath(ep.Pattern)
	if ep.Sensitive && !pathListed(s.Cfg.EndpointInclude, path) {
		log.Debugf(ctx, log.TagAppDef, "endpoint %s disabled: sensitive endpoint not explicitly included", path)
		return false
	}
	if s.Cfg.EndpointInclude != "" && !pathListed(s.Cfg.EndpointInclude, path) {
		log.Debugf(ctx, log.TagAppDef, "endpoint %s disabled: not in include list", path)
		return false
	}
	return true
}

// pathListed reports whether path appears in the comma-separated,
// case-insensitive list. An empty list matches nothing.
func pathListed(list, path string) bool {
	if list == "" {
		return false
	}
	for _, item := range strings.Split(list, ",") {
		if strings.EqualFold(strings.TrimSpace(item), path) {
			return true
		}
	}
	return false
}

// Stop gracefully shuts down the management server, propagating ctx into
// http.Server.Shutdown so the drain rides the shutdown context.
func (s *Server) Stop(ctx context.Context) error {
	if s.svr == nil {
		return nil
	}
	log.Debugf(ctx, log.TagAppDef, "stopping actuator server")
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

// writeJSON writes v as an indented JSON response with the given status code.
func writeJSON(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(code)
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	_ = enc.Encode(v)
}

// handleInfo reports build/version metadata read from the binary's embedded
// build info (module path/version, Go toolchain, and VCS stamp when the binary
// was built from a checkout).
func (s *Server) handleInfo(w http.ResponseWriter, r *http.Request) {
	info := map[string]any{}
	if bi, ok := debug.ReadBuildInfo(); ok {
		info["go"] = bi.GoVersion
		info["module"] = map[string]string{
			"path":    bi.Main.Path,
			"version": bi.Main.Version,
		}
		build := map[string]string{}
		for _, setting := range bi.Settings {
			switch setting.Key {
			case "vcs.revision":
				build["revision"] = setting.Value
			case "vcs.time":
				build["time"] = setting.Value
			case "vcs.modified":
				build["modified"] = setting.Value
			}
		}
		if len(build) > 0 {
			info["build"] = build
		}
	}
	writeJSON(w, http.StatusOK, info)
}
