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
//	          AND every readiness-group indicator passes; 503 otherwise. During
//	          graceful shutdown it flips to 503 OUT_OF_SERVICE (see PreStop) so
//	          Kubernetes drains the pod before servers stop.
//	/startupz (alias /startup)   startup: 503 until the app has finished starting
//	          AND every startup-group indicator passes, then 200. Backs a K8s
//	          startupProbe so a slow boot is not killed by the liveness probe.
//	          Unaffected by drain (startupProbe stops after first success).
//
// Introspection endpoints (all JSON unless noted):
//
//	GET  /info        build/version info from the embedded build metadata.
//	GET  /loggers     configured loggers with their effective levels, plus the
//	                  selectable level names.
//	POST /loggers/{name}  set a logger's level at runtime; body
//	                  {"configuredLevel":"DEBUG"}. 204 on success.
//	GET  /env         merged configuration properties, secrets masked.
//	GET  /configprops merged configuration as a nested tree, secrets masked.
//	GET  /threaddump  goroutine stack dump (text/plain), the Go analogue of a
//	                  JVM thread dump.
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
	"sync/atomic"
	"time"

	"go-spring.org/spring/gs"
	"go-spring.org/stdlib/endpoint"
	"go-spring.org/stdlib/errutil"
	"go-spring.org/stdlib/health"
)

func init() {
	// Register the actuator as a gs.Server under a distinct name so it coexists
	// with the application's main HTTP server (which also exports gs.Server).
	// Enabled by default: the endpoints are cheap and the value — K8s probes,
	// registry health checks — is high.
	gs.Provide(&Server{}).
		Name("actuatorServer").
		Condition(gs.OnProperty("spring.actuator.enabled").HavingValue("true").MatchIfMissing()).
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
	// Address is the management listen address. It defaults to :9370, distinct
	// from the main HTTP server (:9090) and the pprof server (127.0.0.1:9981),
	// and binds all interfaces so in-cluster probes can reach it.
	Address string `value:"${spring.actuator.addr:=:9370}"`

	// Indicators are all beans exported as health.Indicator. Optional: an app
	// with no indicators still gets liveness/readiness/info.
	Indicators []health.Indicator `autowire:"?"`

	// Endpoints are all beans exported as endpoint.Endpoint. Optional: a
	// component (e.g. starter-otel's Prometheus /metrics) contributes its handler
	// here and it is mounted on this same management port, so operators scrape
	// one port instead of each component running its own server. The actuator
	// does not import those components — the seam is the stdlib interface.
	Endpoints []endpoint.Endpoint `autowire:"?"`

	// Env exposes a read-only snapshot of the merged configuration for the /env
	// and /configprops endpoints. Optional so the actuator still builds probes
	// and info when property introspection is unavailable.
	Env *gs.EnvProvider `autowire:"?"`

	svr      *http.Server
	ready    atomic.Bool
	draining atomic.Bool
}

// Run binds the management listener and begins serving immediately. It
// contributes to the application readiness aggregate via sig, and flips its own
// readiness flag once every server (including this one) has reported ready.
func (s *Server) Run(ctx context.Context, sig gs.ReadySignal) error {
	ln, err := net.Listen("tcp", s.Address)
	if err != nil {
		return errutil.Explain(err, "actuator: failed to listen on %s", s.Address)
	}

	mux := http.NewServeMux()
	// Probe endpoints: z-suffixed canonical paths plus legacy aliases.
	mux.HandleFunc("GET /healthz", s.handleLiveness)
	mux.HandleFunc("GET /health", s.handleLiveness)
	mux.HandleFunc("GET /readyz", s.handleReadiness)
	mux.HandleFunc("GET /readiness", s.handleReadiness)
	mux.HandleFunc("GET /startupz", s.handleStartup)
	mux.HandleFunc("GET /startup", s.handleStartup)
	// Introspection endpoints.
	mux.HandleFunc("GET /info", s.handleInfo)
	mux.HandleFunc("GET /loggers", s.handleLoggers)
	mux.HandleFunc("POST /loggers/{name}", s.handleSetLogger)
	mux.HandleFunc("GET /env", s.handleEnv)
	mux.HandleFunc("GET /configprops", s.handleConfigProps)
	mux.HandleFunc("GET /threaddump", s.handleThreadDump)
	// Mount every contributed endpoint (e.g. otel's Prometheus /metrics) on the
	// same management port. Each owns its full path; they are registered after
	// the built-ins so a contributor cannot shadow /health etc. (ServeMux panics
	// on a duplicate pattern, surfacing a misconfiguration at startup).
	for _, ep := range s.Endpoints {
		mux.Handle(ep.Path(), ep)
	}
	s.svr = &http.Server{
		Handler:           mux,
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
		return nil
	}
	return errutil.Explain(err, "actuator: failed to serve on %s", s.Address)
}

// Stop gracefully shuts down the management server.
func (s *Server) Stop() error {
	if s.svr == nil {
		return nil
	}
	return s.svr.Shutdown(context.Background())
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

// decodeJSON decodes a request body into v, rejecting unknown fields and bodies
// larger than 64 KiB so a malformed or oversized POST cannot exhaust memory.
func decodeJSON(r *http.Request, v any) error {
	dec := json.NewDecoder(http.MaxBytesReader(nil, r.Body, 64<<10))
	dec.DisallowUnknownFields()
	return dec.Decode(v)
}
