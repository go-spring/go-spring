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
//	GET  /loggers     configured loggers with their effective levels, plus the
//	                  selectable level names. Read-only: a runtime level override
//	                  (POST /loggers/{name}) is intentionally not implemented.
//	GET  /env         merged configuration properties, secrets masked.
//	GET  /configprops merged configuration as a nested tree, secrets masked.
//	GET  /threaddump  goroutine stack dump (text/plain), the Go analogue of a
//	                  JVM thread dump.
//	GET  /beans       the container's bean list [{name, type}]. The gs core
//	                  does not export bean enumeration, so this reads a
//	                  contributed BeanLister bean; without one it reports that
//	                  boundary (see beans.go).
//
// The introspection endpoints are gated by spring.actuator.endpoints.include /
// .exclude. Sensitive introspection endpoints (/env, /configprops, /threaddump,
// /loggers, /beans) are additionally OFF by default: they expose configuration
// and internals, so they register only when explicitly listed in .include (an
// empty include is no longer a free pass for them). /info (build metadata) and
// contributed endpoints such as /metrics stay default-on; a non-empty include
// is a whitelist for them, and exclude always applies. The probe endpoints are
// always registered — filtering them would break the Kubernetes contract.
//
// The whole management port can be authenticated with
// spring.actuator.token (bearer) or spring.actuator.username /
// spring.actuator.password (HTTP Basic) via the shared stdlib/httpauth guard.
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
	"errors"
	"net"
	"net/http"
	"runtime/debug"
	"strings"
	"sync/atomic"
	"time"

	"go-spring.org/cloud/actuator/endpoint"
	"go-spring.org/cloud/actuator/health"
	"go-spring.org/log"
	"go-spring.org/spring/gs"
	"go-spring.org/stdlib/errutil"
	"go-spring.org/stdlib/httpauth"
)

func init() {
	// Mark that a management server collecting endpoint.Endpoint beans is
	// linked in, so contributors (e.g. starter-otel's Prometheus /metrics with
	// metrics.port=0) can WARN at startup when they would otherwise be
	// silently homeless. See endpoint.MarkServing.
	endpoint.MarkServing()

	// Register the actuator as a gs.Server under a distinct name so it coexists
	// with the application's main HTTP server (which also exports gs.Server).
	// Enabled by default: the endpoints are cheap and the value — K8s probes,
	// registry health checks — is high.
	gs.Provide(&Server{}).
		Condition(gs.OnProperty("spring.actuator.addr")).
		Export(gs.As[gs.Server]())
}

// checkTimeout bounds a single /readiness sweep across all indicators so one
// slow dependency cannot stall the probe past a typical probe timeout.
const checkTimeout = 3 * time.Second

// Server serves the actuator endpoints on a dedicated management port.
//
// The exported fields are populated by the IoC container: Address from
// configuration and Indicators by collecting every bean exported as
// health.Indicator (autowire:"?" makes the set optional, so the actuator works
// with no indicators registered).
type Server struct {
	// Address is the management listen address. There is no default; setting
	// this key is what activates the starter. Documented layout: main HTTP
	// server (:9090), actuator (:9370, all interfaces so in-cluster probes can
	// reach it), pprof (127.0.0.1:9981).
	Address string `value:"${spring.actuator.addr}"`

	// Indicators are all beans exported as health.Indicator. Optional: an app
	// with no indicators still gets liveness/readiness/info.
	Indicators []health.Indicator `autowire:"?"`

	// Endpoints are all beans exported as endpoint.Endpoint. Optional: a
	// component (e.g. starter-otel's Prometheus /metrics) contributes its handler
	// here and it is mounted on this same management port, so operators scrape
	// one port instead of each component running its own server. The actuator
	// does not import those components — the seam is the stdlib interface.
	Endpoints []endpoint.Endpoint `autowire:"?"`

	// Config exposes a read-only snapshot of the merged configuration (via
	// PropertiesRefresher.Snapshot) for the /env and /configprops endpoints.
	// Optional so the actuator still builds probes and info when property
	// introspection is unavailable.
	Config *gs.PropertiesRefresher `autowire:"?"`

	// BeanRegistry optionally supplies the container's bean list for the
	// /beans endpoint. The gs core deliberately does not export bean
	// enumeration (the global registry lives in an internal package and is
	// cleared after wiring), so no default implementation exists; see
	// beans.go for the exact boundary. When nil, /beans reports the boundary
	// instead of an empty (misleading) list.
	BeanRegistry BeanLister `autowire:"?"`

	// EndpointInclude is the comma-separated endpoint whitelist
	// (spring.actuator.endpoints.include). When non-empty, only the named
	// endpoints are registered (whitelist mode). Endpoint names are the path
	// without the leading slash: info, loggers, env, configprops, threaddump,
	// beans, and a contributed endpoint's own path (e.g. "metrics"). Probe
	// endpoints (/healthz, /readyz, /startupz and their aliases) are always
	// registered: filtering them would break the Kubernetes contract.
	// Sensitive endpoints (env, configprops, threaddump, loggers, beans) are
	// default-off and require explicit inclusion here even when this list is
	// empty; listing them in exclude always wins.
	EndpointInclude string `value:"${spring.actuator.endpoints.include:=}"`

	// EndpointExclude is the comma-separated endpoint blacklist
	// (spring.actuator.endpoints.exclude). It always applies, including in
	// whitelist mode: include first selects, exclude then removes.
	EndpointExclude string `value:"${spring.actuator.endpoints.exclude:=}"`

	// Token, when set, requires an "Authorization: Bearer <token>" header on
	// every request to the management port (spring.actuator.token). Takes
	// precedence over Username/Password.
	Token string `value:"${spring.actuator.token:=}"`

	// Username and Password, when both set, require HTTP Basic authentication
	// on the management port (spring.actuator.username /
	// spring.actuator.password).
	Username string `value:"${spring.actuator.username:=}"`
	Password string `value:"${spring.actuator.password:=}"`

	svr      *http.Server
	ready    atomic.Bool
	draining atomic.Bool
}

// route pairs an introspection endpoint's HTTP pattern with the filter name it
// is registered under (the path without the leading slash).
type route struct {
	name    string
	pattern string
	handler http.HandlerFunc
}

// introspectionRoutes lists the actuator's built-in introspection endpoints.
// They are registered only when the include/exclude filter admits them; the
// probe endpoints (/healthz, /readyz, /startupz and aliases) are deliberately
// not in this table — they are always registered so the Kubernetes contract
// cannot be broken by a filtering typo.
func (s *Server) introspectionRoutes() []route {
	return []route{
		{"info", "GET /info", s.handleInfo},
		{"loggers", "GET /loggers", s.handleLoggers},
		{"env", "GET /env", s.handleEnv},
		{"configprops", "GET /configprops", s.handleConfigProps},
		{"threaddump", "GET /threaddump", s.handleThreadDump},
		{"beans", "GET /beans", s.handleBeans},
	}
}

// sensitiveEndpoints names the introspection endpoints that expose
// configuration or internals and therefore do NOT register by default: they
// must be explicitly listed in spring.actuator.endpoints.include. /info is not
// sensitive (build metadata only); probes are exempt from filtering entirely.
var sensitiveEndpoints = map[string]bool{
	"env":         true,
	"configprops": true,
	"threaddump":  true,
	"loggers":     true,
	"beans":       true,
}

// endpointEnabled reports whether the named endpoint passes the include/exclude
// filter. Sensitive endpoints require explicit inclusion even when include is
// empty (default-off). Otherwise, include non-empty means whitelist mode:
// everything not listed is off. exclude always applies, including inside a
// whitelist. Matching is exact and case-insensitive on the endpoint name.
func (s *Server) endpointEnabled(ctx context.Context, name string) bool {
	if sensitiveEndpoints[name] && !nameListed(s.EndpointInclude, name) {
		log.Debugf(ctx, log.TagAppDef, "endpoint %s disabled: sensitive endpoint not explicitly included", name)
		return false
	}
	if s.EndpointInclude != "" && !nameListed(s.EndpointInclude, name) {
		log.Debugf(ctx, log.TagAppDef, "endpoint %s disabled: not in include list", name)
		return false
	}
	if nameListed(s.EndpointExclude, name) {
		log.Debugf(ctx, log.TagAppDef, "endpoint %s disabled: in exclude list", name)
		return false
	}
	return true
}

// nameListed reports whether name appears in the comma-separated,
// case-insensitive list. An empty list matches nothing.
func nameListed(list, name string) bool {
	if list == "" {
		return false
	}
	for _, item := range strings.Split(list, ",") {
		if strings.EqualFold(strings.TrimSpace(item), name) {
			return true
		}
	}
	return false
}

// Run binds the management listener and begins serving immediately. It
// contributes to the application readiness aggregate via sig, and flips its own
// readiness flag once every server (including this one) has reported ready.
func (s *Server) Run(ctx context.Context, sig gs.ReadySignal) error {
	ln, err := net.Listen("tcp", s.Address)
	if err != nil {
		log.Errorf(ctx, log.TagAppDef, "failed to listen on %s: %v", s.Address, err)
		return errutil.Explain(err, "actuator: failed to listen on %s", s.Address)
	}

	log.Infof(ctx, log.TagAppDef, "actuator listening on %s", s.Address)

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
	if errors.Is(err, http.ErrServerClosed) {
		log.Infof(ctx, log.TagAppDef, "actuator server closed gracefully")
		return nil
	}
	log.Errorf(ctx, log.TagAppDef, "actuator serve error: %v", err)
	return errutil.Explain(err, "actuator: failed to serve on %s", s.Address)
}

// buildHandler assembles the management port's handler: probe endpoints
// (unconditional), introspection endpoints (through the include/exclude
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

	// Introspection endpoints — registered through the include/exclude filter.
	for _, rt := range s.introspectionRoutes() {
		if !s.endpointEnabled(ctx, rt.name) {
			continue
		}
		mux.HandleFunc(rt.pattern, rt.handler)
	}

	// Mount every contributed endpoint (e.g. otel's Prometheus /metrics) on the
	// same management port, subject to the same include/exclude filter (an
	// endpoint's name is its path without the leading slash). Each owns its full
	// path; they are registered after the built-ins so a contributor cannot
	// shadow /health etc. (ServeMux panics on a duplicate pattern, surfacing a
	// misconfiguration at startup).
	for _, ep := range s.Endpoints {
		name := strings.TrimPrefix(ep.Path(), "/")
		if !s.endpointEnabled(ctx, name) {
			continue
		}
		mux.Handle(ep.Path(), ep)
		log.Debugf(ctx, log.TagAppDef, "registered endpoint: %s", ep.Path())
	}

	guard := httpauth.Guard{Token: s.Token, Username: s.Username, Password: s.Password}
	if !guard.Enabled() && !httpauth.IsLoopback(s.Address) {
		log.Warnf(ctx, log.TagAppDef,
			"actuator listening on %q without authentication; set ${spring.actuator.token} or ${spring.actuator.username}/${spring.actuator.password}",
			s.Address)
	}
	return guard.Wrap(mux)
}

// Stop gracefully shuts down the management server.
func (s *Server) Stop() error {
	return s.StopContext(context.Background())
}

// StopContext gracefully shuts down the management server, propagating ctx into
// http.Server.Shutdown so the drain rides the shutdown context.
func (s *Server) StopContext(ctx context.Context) error {
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

// writeJSON writes v as an indented JSON response with the given status code.
func writeJSON(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(code)
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	_ = enc.Encode(v)
}
