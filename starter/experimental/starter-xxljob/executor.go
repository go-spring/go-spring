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

// executor.go is the executor core: the callback HTTP server (gs.Server) that
// serves the admin's /run /beat /idleBeat /kill /log endpoints, and the
// handler registry the app populates. Task functions run on goroutines with a
// cancellable context so /kill can interrupt a long task; a panic in a task
// is recovered through the shared goutil panic chain.
package StarterXxljob

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"sync"
	"time"

	"go-spring.org/cloud/actuator/health"
	"go-spring.org/log"
	"go-spring.org/spring/gs"
	"go-spring.org/stdlib/goutil"
)

// TaskFunc is one registered xxl-job handler: it runs the job with the raw
// parameter string and returns an error to report failure. Returning nil
// reports success.
type TaskFunc func(ctx context.Context, param string) error

// Executor is the executor bean. It embeds the handler registry and callback
// server; the admin triggers it via /run, keeps it honest via /beat, asks
// whether it is idle via /idleBeat, interrupts via /kill, and reads task logs
// via /log.
type Executor struct {
	cfg Config
	// name is the instance name under "${spring.xxljob}" — used for the
	// health.Indicator name (matching the bean name, like sibling starters).
	name     string
	registry map[string]TaskFunc

	mu      sync.Mutex
	running map[int64][]*runEntry // jobId -> running runs, for /kill & /idleBeat

	srv  *http.Server
	ip   string // this host's outbound address, for registration
	once sync.Once
}

// RegisterHandler registers fn under handler name. It must be called before
// the executor starts serving (names are fixed once the callback server is
// up).
func (e *Executor) RegisterHandler(name string, fn TaskFunc) {
	e.mu.Lock()
	e.registry[name] = fn
	e.mu.Unlock()
}

// newExecutor builds the executor with the handler registry and callback
// server routes.
func newExecutor(ctx *gs.ContextProvider, name string, c Config) (*Executor, error) {
	e := &Executor{
		cfg:      c,
		name:     name,
		registry: map[string]TaskFunc{},
		running:  map[int64][]*runEntry{},
	}
	mux := http.NewServeMux()
	mux.HandleFunc("/run", e.handleRun)
	mux.HandleFunc("/beat", e.handleBeat)
	mux.HandleFunc("/idleBeat", e.handleIdleBeat)
	mux.HandleFunc("/kill", e.handleKill)
	mux.HandleFunc("/log", e.handleLog)
	e.srv = &http.Server{
		Addr:              fmt.Sprintf(":%d", c.Port),
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,
	}
	return e, nil
}

// Run implements gs.Server: register with the admin, then serve callbacks
// until ctx is cancelled.
func (e *Executor) Run(ctx context.Context, sig gs.ReadySignal) error {
	if err := e.prepare(); err != nil {
		return err
	}
	stopRegistry := e.register(ctx)
	defer stopRegistry()
	if sig != nil {
		sig.TriggerAndWait()
	}
	go func() {
		<-ctx.Done()
		_ = e.srv.Shutdown(context.Background())
	}()
	err := e.srv.ListenAndServe()
	if err == http.ErrServerClosed {
		return nil
	}
	return err
}

// Stop implements gs.Server.
func (e *Executor) Stop() error {
	return e.StopContext(context.Background())
}

// StopContext implements gs.Stopper: gracefully shut the callback server down,
// propagating ctx into http.Server.Shutdown so the drain rides the shutdown
// context.
func (e *Executor) StopContext(ctx context.Context) error {
	return e.srv.Shutdown(ctx)
}

// Destroy is the bean destroy path.
func (e *Executor) Destroy() error { return e.Stop() }

// prepare resolves the outbound IP once (for registration) and ensures the
// log dir exists.
func (e *Executor) prepare() error {
	var err error
	e.once.Do(func() {
		e.ip, err = outboundIP()
		if err == nil {
			err = ensureLogDir(e.cfg.LogDir)
		}
	})
	return err
}

// runEntry is one running task, tracked under its jobId so /kill and
// /idleBeat can find it.
type runEntry struct {
	cancel context.CancelFunc
}

// trackRunning records a running task under its jobId. The admin addresses
// running jobs by jobId (/kill, /idleBeat), NOT by the per-trigger logId, so
// the running table is keyed by jobId; logId is only used for the /log and
// /api/callback paths.
func (e *Executor) trackRunning(jobID int, cancel context.CancelFunc) *runEntry {
	entry := &runEntry{cancel: cancel}
	e.mu.Lock()
	e.running[int64(jobID)] = append(e.running[int64(jobID)], entry)
	e.mu.Unlock()
	return entry
}

// untrackRunning removes one finished run of a job, dropping the map entry
// when the last run is gone.
func (e *Executor) untrackRunning(jobID int, entry *runEntry) {
	e.mu.Lock()
	list := e.running[int64(jobID)]
	for i, en := range list {
		if en == entry {
			list = append(list[:i], list[i+1:]...)
			break
		}
	}
	if len(list) == 0 {
		delete(e.running, int64(jobID))
	} else {
		e.running[int64(jobID)] = list
	}
	e.mu.Unlock()
}

// handleRun runs a task in a new goroutine and returns immediately; the task
// posts its completion back to the admin via /api/callback (see registry.go).
func (e *Executor) handleRun(w http.ResponseWriter, r *http.Request) {
	var p TriggerParam
	if err := json.NewDecoder(r.Body).Decode(&p); err != nil {
		writeTriggerResponse(w, 500, "bad trigger param")
		return
	}
	fn, ok := e.registry[p.ExecutorHandler]
	if !ok {
		writeTriggerResponse(w, 500, "no handler registered for "+p.ExecutorHandler)
		return
	}
	ctx, cancel := context.WithCancel(context.Background())
	entry := e.trackRunning(p.JobID, cancel)

	go func() {
		defer func() {
			e.untrackRunning(p.JobID, entry)
			cancel()
		}()
		err := goutil.SafeRun(ctx, func(ctx context.Context) error {
			return fn(ctx, p.ExecutorParams)
		})
		res := handlerResult{code: 200}
		if err != nil {
			res.code = 500
			res.msg = err.Error()
		}
		// Report through a context that survives a /kill cancellation, so the
		// admin still gets the failure callback for a killed job.
		cbCtx, cbCancel := context.WithTimeout(context.WithoutCancel(ctx), 10*time.Second)
		defer cbCancel()
		e.callback(cbCtx, p.LogID, p.LogDateTime, res)
	}()
	writeTriggerResponse(w, 200, "")
}

// handleBeat answers "are you alive".
func (e *Executor) handleBeat(w http.ResponseWriter, _ *http.Request) {
	writeTriggerResponse(w, 200, "")
}

// handleIdleBeat answers "are you idle" — the admin uses this for
// block-strategy decisions (SERIAL_EXECUTION etc). The admin POSTs an
// IdleBeatParam JSON body keyed by jobId (not logId).
func (e *Executor) handleIdleBeat(w http.ResponseWriter, r *http.Request) {
	var p IdleBeatParam
	if err := json.NewDecoder(r.Body).Decode(&p); err != nil {
		writeTriggerResponse(w, 500, "bad idleBeat param")
		return
	}
	e.mu.Lock()
	_, running := e.running[p.JobID]
	e.mu.Unlock()
	if running {
		writeTriggerResponse(w, 500, "job running")
		return
	}
	writeTriggerResponse(w, 200, "")
}

// handleKill cancels a running task's context. The admin POSTs a KillParam
// JSON body keyed by jobId, so the running table is looked up by jobId —
// logId differs per trigger and would never match.
func (e *Executor) handleKill(w http.ResponseWriter, r *http.Request) {
	var p KillParam
	if err := json.NewDecoder(r.Body).Decode(&p); err != nil {
		writeTriggerResponse(w, 500, "bad kill param")
		return
	}
	e.mu.Lock()
	list := append([]*runEntry(nil), e.running[p.JobID]...)
	e.mu.Unlock()
	if len(list) > 0 {
		for _, entry := range list {
			entry.cancel()
		}
		writeTriggerResponse(w, 200, "")
		return
	}
	writeTriggerResponse(w, 500, "job not running")
}

// handleLog serves the task log file back to the admin.
func (e *Executor) handleLog(w http.ResponseWriter, r *http.Request) {
	logID := r.URL.Query().Get("logId")
	fromLine, _ := strconv.Atoi(r.URL.Query().Get("fromLineNum"))
	content, toLine, end, err := readLog(e.cfg.LogDir, logID, fromLine)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	_ = json.NewEncoder(w).Encode(LogResult{
		FromLineNum: fromLine, ToLineNum: toLine, LogContent: content, IsEnd: end,
	})
}

// callback POSTs the task outcome to the admin's /api/callback. The official
// admin expects a JSON array of HandleCallbackParam (logId/logDateTime/
// handleCode/handleMsg), NOT the LogResult shape used by the /log endpoint.
func (e *Executor) callback(ctx context.Context, logID, logDateTime int64, res handlerResult) {
	body, _ := json.Marshal([]HandleCallbackParam{{
		LogID:       logID,
		LogDateTime: logDateTime,
		HandleCode:  res.code,
		HandleMsg:   res.msg,
	}})
	for _, base := range e.cfg.AdminAddresses {
		req, err := http.NewRequestWithContext(ctx, http.MethodPost,
			base+"/api/callback", bytes.NewReader(body))
		if err != nil {
			continue
		}
		req.Header.Set("Content-Type", "application/json")
		if e.cfg.AccessToken != "" {
			req.Header.Set("XXL-JOB-ACCESS-TOKEN", e.cfg.AccessToken)
		}
		if resp, err := http.DefaultClient.Do(req); err == nil {
			_ = resp.Body.Close()
		}
	}
}

// Health returns an indicator that reports whether the callback server is
// serving (the executor is ready). Named after the instance name, matching
// the bean name in starter.go (sibling-starters convention).
func (e *Executor) Health() *health.Indicator {
	return &health.Indicator{Name: "xxljob:"+e.name, Probe: func(ctx context.Context) error {
		if e.srv == nil {
			return fmt.Errorf("xxljob: executor not started")
		}
		return nil
	}}
}

func writeTriggerResponse(w http.ResponseWriter, code int, msg string) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(TriggerResponse{Code: code, Msg: msg})
}

var _ = log.TagAppDef
