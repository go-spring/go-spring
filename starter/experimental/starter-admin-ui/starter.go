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

// Package StarterAdminUI is the "Spring Boot Admin equivalent" for Go-Spring:
// a lightweight, self-contained HTML dashboard that periodically polls the
// starter-actuator endpoints (/health, /readiness, /startup, /info) of a
// configured list of application instances and renders an aggregated status
// table.
//
// It is deliberately narrow in scope. In a Kubernetes deployment the standard
// aggregate-monitoring story is Prometheus + Grafana (see contrib/observability
// in this repo) — that path gives history, alerting, and rich dashboards, and
// is the recommended default. This starter targets the case where that stack
// is unavailable or overkill: on-prem clusters without Prometheus, ad-hoc
// bring-up of a new environment, or quick local inspection of a handful of
// pods. Framing it in the spirit of "equivalent effect via idiomatic Go" — not
// a feature-for-feature reimplementation of Spring Boot Admin — the dashboard
// is one HTML page, one poller goroutine, zero third-party dependencies.
//
// Form: Server. The starter self-hosts its own HTTP listener on a dedicated
// port (:9280 by default) and exports a gs.Server bean; it participates in
// the framework's ready signal and graceful shutdown just like starter-actuator
// and starter-pprof. See starter/DESIGN_CN.md §2.1.
package StarterAdminUI

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"html/template"
	"net"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"sync"
	"time"

	"go-spring.org/spring/gs"
	"go-spring.org/stdlib/errutil"
)

func init() {
	// Register the Admin UI as a distinct gs.Server so it coexists with the
	// main HTTP server, the actuator, and pprof. Default-on: like starter-
	// actuator, the value is high (a one-glance view of the cluster) and the
	// cost is a single idle goroutine when Instances is empty.
	gs.Provide(&Server{}).
		Name("adminUIServer").
		Condition(gs.OnProperty("spring.admin-ui.enabled").HavingValue("true").MatchIfMissing()).
		Export(gs.As[gs.Server]())
}

// Server serves the Admin UI dashboard and drives the background poller.
//
// The Config field is populated by the IoC container from the "spring.admin-ui"
// property tree. The Server also holds the runtime state — the poller
// goroutine's lifecycle, the HTTP server, and the latest snapshot behind a
// mutex — that must not be exposed as configuration.
type Server struct {
	Config Config `value:"${spring.admin-ui}"`

	svr    *http.Server
	tpl    *template.Template
	client *http.Client

	// stop is closed by Stop() to unblock the poller loop.
	stop chan struct{}
	// done is closed by the poller when it has exited, so Stop() can wait.
	done chan struct{}

	// mu guards snapshot. The HTTP handler reads it; the poller writes it.
	// The read path never blocks on a live poll — a stale snapshot beats a
	// slow page load.
	mu       sync.RWMutex
	snapshot []instanceStatus
	polledAt time.Time
}

// Run binds the listener, starts the poller, and serves the dashboard.
//
// The order matters: bind first (so a port conflict fails fast during
// startup, before the readiness barrier), then TriggerAndWait (so the app
// can proceed past its own readiness), then Serve. Once Serve returns
// gracefully via Stop, ErrServerClosed is swallowed.
func (s *Server) Run(ctx context.Context, sig gs.ReadySignal) error {
	ln, err := net.Listen("tcp", s.Config.Addr)
	if err != nil {
		return errutil.Explain(err, "admin-ui: failed to listen on %s", s.Config.Addr)
	}

	// Pre-parse the template once. A parse failure here would indicate a bug
	// in the embedded template string, not a runtime input, so panicking is
	// appropriate — but returning an error keeps startup diagnostics uniform.
	tpl, err := template.New("dashboard").Funcs(templateFuncs).Parse(dashboardTemplate)
	if err != nil {
		return errutil.Explain(err, "admin-ui: failed to parse dashboard template")
	}
	s.tpl = tpl

	// One HTTP client shared across polls: connection reuse matters when the
	// same actuator is polled every Interval. The per-request timeout is
	// enforced through context, not Client.Timeout, so the sweep can cancel
	// the whole cascade cleanly on shutdown.
	s.client = &http.Client{
		Transport: &http.Transport{
			MaxIdleConns:        len(s.Config.Instances) * 4,
			MaxIdleConnsPerHost: 4,
			IdleConnTimeout:     90 * time.Second,
		},
	}

	s.stop = make(chan struct{})
	s.done = make(chan struct{})

	// Seed the snapshot synchronously so the first page load — which may
	// arrive before the first tick — shows real data rather than an empty
	// table. Bounded by the poll timeout, so this cannot delay startup.
	s.refresh(ctx)

	mux := http.NewServeMux()
	mux.HandleFunc("GET /", s.handleDashboard)
	mux.HandleFunc("GET /api/status", s.handleStatusJSON)

	s.svr = &http.Server{
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,
	}

	// Trigger readiness (participate in the aggregate) but do NOT block on
	// the returned channel: the dashboard should be reachable during startup
	// so operators can watch the transition, just like the actuator.
	_ = sig.TriggerAndWait()

	go s.pollLoop()

	err = s.svr.Serve(ln)
	if errors.Is(err, http.ErrServerClosed) {
		return nil
	}
	return errutil.Explain(err, "admin-ui: failed to serve on %s", s.Config.Addr)
}

// Stop shuts the server down gracefully and waits for the poller to exit.
func (s *Server) Stop() error {
	if s.stop != nil {
		// Idempotent close guard — Stop is called at most once by the framework,
		// but a nil-safety close is cheap and defensive.
		select {
		case <-s.stop:
		default:
			close(s.stop)
		}
	}
	if s.done != nil {
		<-s.done
	}
	if s.svr == nil {
		return nil
	}
	return s.svr.Shutdown(context.Background())
}

// pollLoop refreshes the snapshot on a fixed cadence until Stop closes the
// stop channel. Each refresh runs on a bounded context so a wedged instance
// cannot delay the next tick beyond one Interval.
func (s *Server) pollLoop() {
	defer close(s.done)

	// A zero or negative interval would busy-loop; fall back to a sane default
	// rather than crashing, since Interval is a runtime configuration value.
	interval := s.Config.Interval
	if interval <= 0 {
		interval = 10 * time.Second
	}
	t := time.NewTicker(interval)
	defer t.Stop()

	for {
		select {
		case <-s.stop:
			return
		case <-t.C:
			// Cap the total sweep at one interval so a slow set of instances
			// never lets consecutive sweeps overlap.
			ctx, cancel := context.WithTimeout(context.Background(), interval)
			s.refresh(ctx)
			cancel()
		}
	}
}

// refresh polls every configured instance in parallel and stores the result
// as the new snapshot. It never partially updates: either every instance was
// visited within ctx's deadline or the sweep publishes what it managed to
// collect (unreachable instances show up as DOWN with the error message).
func (s *Server) refresh(ctx context.Context) {
	instances := s.Config.Instances
	timeout := s.Config.Timeout
	if timeout <= 0 {
		timeout = 3 * time.Second
	}

	results := make([]instanceStatus, len(instances))
	var wg sync.WaitGroup
	for i, base := range instances {
		wg.Add(1)
		go func(i int, base string) {
			defer wg.Done()
			results[i] = s.pollOne(ctx, base, timeout)
		}(i, base)
	}
	wg.Wait()

	// Stable order so the table doesn't shuffle between refreshes. Sort by
	// base URL because that's what the operator identifies rows by.
	sort.Slice(results, func(i, j int) bool { return results[i].Base < results[j].Base })

	s.mu.Lock()
	s.snapshot = results
	s.polledAt = time.Now()
	s.mu.Unlock()
}

// instanceStatus is the per-row payload the template and the JSON API render.
// It intentionally flattens the actuator response into UI-friendly primitives —
// the actuator's shape is authoritative but noisy for a compact table.
type instanceStatus struct {
	Base       string            `json:"base"`
	Health     string            `json:"health"`
	Readiness  string            `json:"readiness"`
	Startup    string            `json:"startup"`
	Components []componentStatus `json:"components,omitempty"`
	GoVersion  string            `json:"go,omitempty"`
	Module     string            `json:"module,omitempty"`
	Version    string            `json:"version,omitempty"`
	Revision   string            `json:"revision,omitempty"`
	BuildTime  string            `json:"build_time,omitempty"`
	Error      string            `json:"error,omitempty"`
}

// componentStatus mirrors the per-indicator entry returned by /readiness.
type componentStatus struct {
	Name   string `json:"name"`
	Status string `json:"status"`
	Error  string `json:"error,omitempty"`
}

// pollOne fetches the four actuator endpoints for a single instance. Each call
// is independent — a broken /info doesn't mask a working /health — so the row
// can show partial data even under partial failure. The overall Error field is
// only set when even /health could not be reached, which is the operator's cue
// that the instance is unreachable at all.
func (s *Server) pollOne(ctx context.Context, base string, timeout time.Duration) instanceStatus {
	out := instanceStatus{Base: base}

	// Guard against obvious misconfiguration: an empty entry in the Instances
	// slice would otherwise poll the local process, which is confusing and
	// hides the misconfiguration.
	if strings.TrimSpace(base) == "" {
		out.Error = "empty instance URL"
		out.Health = "UNKNOWN"
		return out
	}
	if _, err := url.Parse(base); err != nil {
		out.Error = fmt.Sprintf("invalid URL: %v", err)
		out.Health = "UNKNOWN"
		return out
	}

	base = strings.TrimRight(base, "/")

	// /health — liveness. If this fails, the instance is treated as down.
	if body, err := s.getJSON(ctx, base+"/health", timeout); err != nil {
		out.Health = "DOWN"
		out.Error = err.Error()
	} else {
		out.Health = statusOf(body)
	}

	// /readiness — plus per-component breakdown when present.
	if body, err := s.getJSON(ctx, base+"/readiness", timeout); err != nil {
		out.Readiness = "DOWN"
	} else {
		out.Readiness = statusOf(body)
		if raw, ok := body["components"].(map[string]any); ok {
			names := make([]string, 0, len(raw))
			for name := range raw {
				names = append(names, name)
			}
			sort.Strings(names)
			for _, name := range names {
				entry, _ := raw[name].(map[string]any)
				comp := componentStatus{Name: name, Status: statusOf(entry)}
				if e, _ := entry["error"].(string); e != "" {
					comp.Error = e
				}
				out.Components = append(out.Components, comp)
			}
		}
	}

	// /startup — signals startup-probe completion. Not fatal if missing.
	if body, err := s.getJSON(ctx, base+"/startup", timeout); err != nil {
		out.Startup = "DOWN"
	} else {
		out.Startup = statusOf(body)
	}

	// /info — build metadata. Errors here are silently ignored: build info is
	// nice-to-have, and a 404 from a non-actuator target should not spam the
	// UI. Nested fields are read defensively because the shape is
	// user-controlled.
	if body, err := s.getJSON(ctx, base+"/info", timeout); err == nil {
		if v, ok := body["go"].(string); ok {
			out.GoVersion = v
		}
		if m, ok := body["module"].(map[string]any); ok {
			if v, ok := m["path"].(string); ok {
				out.Module = v
			}
			if v, ok := m["version"].(string); ok {
				out.Version = v
			}
		}
		if b, ok := body["build"].(map[string]any); ok {
			if v, ok := b["revision"].(string); ok {
				out.Revision = v
			}
			if v, ok := b["time"].(string); ok {
				out.BuildTime = v
			}
		}
	}

	return out
}

// getJSON GETs url with a per-request timeout derived from ctx and decodes the
// response body as JSON object. Non-2xx responses that still carry a status
// payload (e.g. 503 from /readiness with body {"status":"OUT_OF_SERVICE"}) are
// treated as success at the transport level so the caller can read the status.
func (s *Server) getJSON(ctx context.Context, url string, timeout time.Duration) (map[string]any, error) {
	reqCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	req, err := http.NewRequestWithContext(reqCtx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	resp, err := s.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	// Accept any status code — /readiness returns 503 with a valid JSON body
	// when a component is down, and we still want to render that status.
	body := make(map[string]any)
	dec := json.NewDecoder(resp.Body)
	if err := dec.Decode(&body); err != nil {
		return nil, errutil.Explain(err, "decode %s (HTTP %d)", url, resp.StatusCode)
	}
	return body, nil
}

// statusOf extracts the "status" field, returning "UNKNOWN" when the field is
// missing or not a string — the actuator contract says it will always be
// there, but a third-party endpoint impersonating actuator shape might not
// comply.
func statusOf(body map[string]any) string {
	if body == nil {
		return "UNKNOWN"
	}
	if v, ok := body["status"].(string); ok && v != "" {
		return v
	}
	return "UNKNOWN"
}

// handleDashboard renders the aggregated view as HTML from the latest snapshot.
// It never blocks on a live poll: the poller is the only writer, this is a
// read-only view.
func (s *Server) handleDashboard(w http.ResponseWriter, r *http.Request) {
	s.mu.RLock()
	snap := make([]instanceStatus, len(s.snapshot))
	copy(snap, s.snapshot)
	polled := s.polledAt
	s.mu.RUnlock()

	interval := s.Config.Interval
	if interval <= 0 {
		interval = 10 * time.Second
	}

	data := struct {
		Title      string
		Instances  []instanceStatus
		PolledAt   string
		RefreshSec int
	}{
		Title:      s.Config.Title,
		Instances:  snap,
		PolledAt:   polled.Format(time.RFC3339),
		RefreshSec: int(interval.Round(time.Second).Seconds()),
	}
	if data.RefreshSec < 1 {
		data.RefreshSec = 1
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := s.tpl.Execute(w, data); err != nil {
		// Log-free path: writing HTML during rendering can fail if the client
		// hangs up, and that's not something to escalate. The template itself
		// is trusted (it's a compile-time constant), so any error here is
		// I/O.
		return
	}
}

// handleStatusJSON exposes the same snapshot as machine-readable JSON so
// scripts, health checkers, and simple integrations can consume it without
// scraping HTML.
func (s *Server) handleStatusJSON(w http.ResponseWriter, r *http.Request) {
	s.mu.RLock()
	snap := make([]instanceStatus, len(s.snapshot))
	copy(snap, s.snapshot)
	polled := s.polledAt
	s.mu.RUnlock()

	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	_ = enc.Encode(map[string]any{
		"polled_at": polled.Format(time.RFC3339),
		"instances": snap,
	})
}

// templateFuncs backs the {{... | statusClass}} pipeline in the HTML template,
// mapping a status string to a CSS class so the color mapping lives in Go and
// the template stays declarative.
var templateFuncs = template.FuncMap{
	"statusClass": func(s string) string {
		switch strings.ToUpper(s) {
		case "UP":
			return "up"
		case "DOWN":
			return "down"
		case "OUT_OF_SERVICE":
			return "oos"
		default:
			return "unknown"
		}
	},
}

// dashboardTemplate is the whole UI. It is embedded as a Go string constant
// on purpose: the starter must be self-contained with no external assets and
// no CDN fetches, so an air-gapped deployment can run it unmodified. Layout
// choices favor legibility over polish — a single centered table with
// color-coded status pills is enough to spot a bad pod at a glance.
const dashboardTemplate = `<!doctype html>
<html lang="en">
<head>
<meta charset="utf-8">
<meta http-equiv="refresh" content="{{.RefreshSec}}">
<title>{{.Title}}</title>
<style>
 body { font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", Helvetica, Arial, sans-serif;
        margin: 24px; background: #fafafa; color: #222; }
 h1 { margin: 0 0 4px 0; font-size: 20px; }
 .meta { color: #888; font-size: 12px; margin-bottom: 16px; }
 table { border-collapse: collapse; width: 100%; background: #fff;
         box-shadow: 0 1px 2px rgba(0,0,0,.06); }
 th, td { text-align: left; padding: 8px 12px; border-bottom: 1px solid #eee;
          vertical-align: top; font-size: 13px; }
 th { background: #f4f4f4; font-weight: 600; }
 .pill { display: inline-block; padding: 2px 8px; border-radius: 10px;
         font-size: 11px; font-weight: 600; color: #fff; }
 .pill.up { background: #2e7d32; }
 .pill.down { background: #c62828; }
 .pill.oos { background: #ef6c00; }
 .pill.unknown { background: #757575; }
 .components { margin: 4px 0 0 0; padding: 0; list-style: none; font-size: 12px; }
 .components li { margin: 2px 0; }
 .build { color: #555; font-size: 12px; }
 .empty { padding: 24px; text-align: center; color: #888; }
</style>
</head>
<body>
<h1>{{.Title}}</h1>
<div class="meta">last polled: {{.PolledAt}} — auto-refresh every {{.RefreshSec}}s</div>
{{if .Instances}}
<table>
 <thead>
  <tr>
   <th>Instance</th>
   <th>Health</th>
   <th>Readiness</th>
   <th>Startup</th>
   <th>Components</th>
   <th>Build</th>
  </tr>
 </thead>
 <tbody>
 {{range .Instances}}
  <tr>
   <td><code>{{.Base}}</code>{{if .Error}}<div class="build">error: {{.Error}}</div>{{end}}</td>
   <td><span class="pill {{.Health | statusClass}}">{{.Health}}</span></td>
   <td><span class="pill {{.Readiness | statusClass}}">{{.Readiness}}</span></td>
   <td><span class="pill {{.Startup | statusClass}}">{{.Startup}}</span></td>
   <td>
    {{if .Components}}
    <ul class="components">
    {{range .Components}}
     <li><span class="pill {{.Status | statusClass}}">{{.Status}}</span> {{.Name}}{{if .Error}} — {{.Error}}{{end}}</li>
    {{end}}
    </ul>
    {{else}}<span class="build">—</span>{{end}}
   </td>
   <td class="build">
    {{if .Module}}<div>{{.Module}} {{.Version}}</div>{{end}}
    {{if .Revision}}<div>rev: {{.Revision}}</div>{{end}}
    {{if .BuildTime}}<div>time: {{.BuildTime}}</div>{{end}}
    {{if .GoVersion}}<div>go: {{.GoVersion}}</div>{{end}}
   </td>
  </tr>
 {{end}}
 </tbody>
</table>
{{else}}
<div class="empty">No instances configured. Set <code>spring.admin-ui.instances</code> to a comma-separated list of actuator base URLs (e.g. <code>http://10.0.0.1:9370</code>).</div>
{{end}}
</body>
</html>
`
