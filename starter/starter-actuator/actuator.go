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

// Package StarterActuator exposes operational HTTP endpoints — liveness,
// readiness, and build info — on a dedicated management port.
//
// Unlike the application's main HTTP server (gs.SimpleHttpServer), which only
// begins serving once every server has signaled readiness, the actuator starts
// serving the moment its listener is bound. This is deliberate: a readiness
// probe must be able to reach the endpoint *before* the app is ready so it can
// observe the OUT_OF_SERVICE -> UP transition, and a liveness probe must answer
// throughout a long startup so the pod is not killed prematurely.
//
// Endpoints (all GET, JSON responses):
//
//	/health     always 200 {"status":"UP"} once the process is serving; it
//	            reflects liveness (the process is up), not dependency health, so
//	            a down database never trips a liveness restart.
//	/readiness  200 {"status":"UP"} only after the app has reported readiness
//	            AND every registered health.Indicator passes; 503 otherwise.
//	/info       200 with build/version info from the embedded build metadata.
//
// Health indicators are contributed by other beans: any bean exported as
// health.Indicator (a redis client wrapper, a gorm pool wrapper, ...) is
// collected here and folded into /readiness with zero per-component wiring.
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

	svr   *http.Server
	ready atomic.Bool
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
	mux.HandleFunc("GET /health", s.handleHealth)
	mux.HandleFunc("GET /readiness", s.handleReadiness)
	mux.HandleFunc("GET /info", s.handleInfo)
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

// handleHealth reports liveness: the process is up and serving. It intentionally
// does not consult health indicators — a degraded dependency should fail
// readiness, not trigger a liveness restart.
func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{"status": health.StatusUp})
}

// componentStatus is the per-indicator entry reported under /readiness.
type componentStatus struct {
	Status health.Status `json:"status"`
	Error  string        `json:"error,omitempty"`
}

// handleReadiness reports whether the app is ready to receive traffic: the
// readiness barrier has been crossed AND every registered indicator passes.
// Returns 503 while not ready or when any required component is down.
func (s *Server) handleReadiness(w http.ResponseWriter, r *http.Request) {
	if !s.ready.Load() {
		writeJSON(w, http.StatusServiceUnavailable, map[string]any{
			"status": "OUT_OF_SERVICE",
		})
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), checkTimeout)
	defer cancel()

	overall := health.StatusUp
	components := make(map[string]componentStatus, len(s.Indicators))
	for _, ind := range s.Indicators {
		if err := ind.CheckHealth(ctx); err != nil {
			components[ind.HealthName()] = componentStatus{Status: health.StatusDown, Error: err.Error()}
			overall = health.StatusDown
		} else {
			components[ind.HealthName()] = componentStatus{Status: health.StatusUp}
		}
	}

	code := http.StatusOK
	if overall == health.StatusDown {
		code = http.StatusServiceUnavailable
	}
	body := map[string]any{"status": overall}
	if len(components) > 0 {
		body["components"] = components
	}
	writeJSON(w, code, body)
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
