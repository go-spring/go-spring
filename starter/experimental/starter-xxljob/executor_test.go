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

package StarterXxljob

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"go-spring.org/stdlib/testing/assert"
)

// newTestExecutor builds an Executor wired to a mock admin at adminURL; the
// admin records the raw /api/callback body for shape assertions.
func newTestExecutor(t *testing.T, adminURL string) (*Executor, *atomic.Value) {
	t.Helper()
	var lastBody atomic.Value
	admin := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/callback" {
			var buf bytes.Buffer
			_, _ = buf.ReadFrom(r.Body)
			lastBody.Store(buf.Bytes())
			_ = json.NewEncoder(w).Encode(map[string]any{"code": 200, "msg": nil})
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(admin.Close)
	if adminURL == "" {
		adminURL = admin.URL
	}
	return &Executor{
		cfg:      Config{AdminAddresses: []string{adminURL}},
		registry: map[string]TaskFunc{},
		running:  map[int64][]*runEntry{},
	}, &lastBody
}

// TestKillByJobIDNotLogID pins the /kill fix: the running table is keyed by
// jobId, so a kill whose jobId differs from the trigger's logId still finds
// and cancels the running task (the old code keyed by logId and never hit).
func TestKillByJobIDNotLogID(t *testing.T) {
	e, _ := newTestExecutor(t, "")
	cancelled := make(chan struct{})
	e.registry["sleepJob"] = func(ctx context.Context, _ string) error {
		<-ctx.Done()
		close(cancelled)
		return ctx.Err()
	}

	// Trigger jobId=2 with logId=2002 — different keys on purpose.
	body, _ := json.Marshal(TriggerParam{JobID: 2, LogID: 2002, ExecutorHandler: "sleepJob"})
	rec := httptest.NewRecorder()
	e.handleRun(rec, httptest.NewRequest(http.MethodPost, "/run", bytes.NewReader(body)))
	assert.That(t, rec.Code).Equal(http.StatusOK)

	// idleBeat by jobId must see the job running.
	rec = httptest.NewRecorder()
	ib, _ := json.Marshal(IdleBeatParam{JobID: 2})
	e.handleIdleBeat(rec, httptest.NewRequest(http.MethodPost, "/idleBeat", bytes.NewReader(ib)))
	var idle TriggerResponse
	_ = json.Unmarshal(rec.Body.Bytes(), &idle)
	assert.That(t, idle.Code).Equal(500)

	// kill by jobId (not logId) must cancel the task.
	rec = httptest.NewRecorder()
	kb, _ := json.Marshal(KillParam{JobID: 2})
	e.handleKill(rec, httptest.NewRequest(http.MethodPost, "/kill", bytes.NewReader(kb)))
	var kill TriggerResponse
	_ = json.Unmarshal(rec.Body.Bytes(), &kill)
	assert.That(t, kill.Code).Equal(200)

	select {
	case <-cancelled:
	case <-time.After(5 * time.Second):
		t.Fatal("kill by jobId did not cancel the task")
	}

	// After the run finishes, idleBeat must report idle again.
	time.Sleep(100 * time.Millisecond)
	rec = httptest.NewRecorder()
	e.handleIdleBeat(rec, httptest.NewRequest(http.MethodPost, "/idleBeat", bytes.NewReader(ib)))
	_ = json.Unmarshal(rec.Body.Bytes(), &idle)
	assert.That(t, idle.Code).Equal(200)
}

// TestCallbackPayloadShape pins the /api/callback body to the official
// HandleCallbackParam array shape, with logId carried through from the
// trigger even though the task was keyed by jobId.
func TestCallbackPayloadShape(t *testing.T) {
	e, lastBody := newTestExecutor(t, "")
	done := make(chan struct{})
	e.registry["demoJob"] = func(context.Context, string) error { close(done); return nil }

	body, _ := json.Marshal(TriggerParam{JobID: 7, LogID: 4242, LogDateTime: 1700000000000, ExecutorHandler: "demoJob"})
	rec := httptest.NewRecorder()
	e.handleRun(rec, httptest.NewRequest(http.MethodPost, "/run", bytes.NewReader(body)))
	<-done

	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if raw, ok := lastBody.Load().([]byte); ok && len(raw) > 0 {
			var arr []HandleCallbackParam
			assert.Error(t, json.Unmarshal(raw, &arr)).Nil()
			assert.That(t, len(arr)).Equal(1)
			assert.That(t, arr[0].LogID).Equal(int64(4242))
			assert.That(t, arr[0].LogDateTime).Equal(int64(1700000000000))
			assert.That(t, arr[0].HandleCode).Equal(200)
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("callback body never arrived")
}

// TestHealthIndicatorNamedByInstance pins the instance-name convention: the
// indicator display name must be "xxljob:"+<instance name> (matching the bean
// name in starter.go), not the app-name — the two diverge when they differ.
func TestHealthIndicatorNamedByInstance(t *testing.T) {
	e, err := newExecutor(nil, "instance-a", Config{AppName: "some-app"})
	assert.Error(t, err).Nil()
	ind := e.Health()
	assert.String(t, ind.Name).Equal("xxljob:instance-a")
}
