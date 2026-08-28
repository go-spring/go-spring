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

// Package main is the self-contained example for starter-xxljob. It starts a
// tiny mock xxl-job admin (just enough of the REST surface to register an
// executor and trigger a run), starts the executor, registers a handler,
// triggers the job through the mock admin, and asserts the handler ran. No
// real xxl-job-admin needed — the protocol under test is the executor's side.
package main

import (
	"bytes"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"sync"
	"syscall"
	"time"

	"go-spring.org/log"
	"go-spring.org/spring/gs"

	starter "go-spring.org/starter-xxljob"
)

const adminAddr = "127.0.0.1:18081"

// handlerRan signals a successful task execution.
var handlerRan = make(chan struct{}, 1)

// sleepDone signals the long-running job observed its /kill cancellation.
var sleepDone = make(chan struct{}, 1)

// callbacks records every body POSTed to the mock admin's /api/callback.
var callbacks = struct {
	sync.Mutex
	bodies [][]byte
}{}

type Service struct {
	Executor *starter.Executor `autowire:"a"`
}

// Init registers the handlers after the executor bean is injected.
func (s *Service) Init() error {
	s.Executor.RegisterHandler("demoJob", func(ctx context.Context, param string) error {
		if param != "a=1" {
			return fmt.Errorf("unexpected param %q", param)
		}
		select {
		case handlerRan <- struct{}{}:
		default:
		}
		return nil
	})
	// sleepJob blocks until /kill cancels its context — the kill proof.
	s.Executor.RegisterHandler("sleepJob", func(ctx context.Context, _ string) error {
		<-ctx.Done()
		select {
		case sleepDone <- struct{}{}:
		default:
		}
		return ctx.Err()
	})
	return nil
}

var manual = flag.Bool("manual", false, "run in manual verification mode (server stays up)")

func main() {
	flag.Parse()

	// Mock admin: /api/registry (accept register/heartbeat), /api/registry/remove,
	// and /api/trigger which calls back into the executor's /run.
	go mockAdmin()

	svrBean := gs.Provide(&Service{}).Export(gs.As[gs.Rooter]()).Init((*Service).Init)

	if !*manual {
		go func() {
			time.Sleep(1 * time.Second)
			runTest(svrBean.Interface().(*Service))
		}()
	} else {
		fmt.Println("=== Manual verification mode ===")
		fmt.Println("Server is running. Follow the README commands in another terminal.")
		fmt.Println("Press Ctrl+C to stop.")
	}
	gs.Run()
}

// mockAdmin serves just enough xxl-job admin REST to exercise the executor:
// registry endpoints no-op, /api/callback records the executor's completion
// payload, and /trigger POSTs a TriggerParam to the executor's /run and
// returns its response.
func mockAdmin() {
	mux := http.NewServeMux()
	mux.HandleFunc("/api/registry", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	mux.HandleFunc("/api/registry/remove", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	mux.HandleFunc("/api/callback", func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		callbacks.Lock()
		callbacks.bodies = append(callbacks.bodies, body)
		callbacks.Unlock()
		_ = json.NewEncoder(w).Encode(map[string]any{"code": 200, "msg": nil})
	})
	mux.HandleFunc("/api/trigger", func(w http.ResponseWriter, r *http.Request) {
		var param starter.TriggerParam
		if err := json.NewDecoder(r.Body).Decode(&param); err != nil || param.ExecutorHandler == "" {
			param = starter.TriggerParam{
				JobID: 1, ExecutorHandler: "demoJob", ExecutorParams: "a=1", LogID: 1001,
				ExecutorTimeout: 30, LogDateTime: time.Now().UnixMilli(),
			}
		}
		body, _ := json.Marshal(param)
		resp, err := http.Post("http://127.0.0.1:9999/run", "application/json", bytes.NewReader(body))
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		defer resp.Body.Close()
		_ = json.NewEncoder(w).Encode(map[string]int{"code": 200})
	})
	_ = http.ListenAndServe(adminAddr, mux)
}

func runTest(s *Service) {
	ctx := context.Background()

	resp, err := http.Post("http://"+adminAddr+"/api/trigger", "application/json", nil)
	if err != nil {
		log.Errorf(ctx, log.TagAppDef, "TRIGGER failed: %v", err)
		os.Exit(1)
	}
	_ = resp.Body.Close()

	select {
	case <-handlerRan:
		fmt.Println("xxl-job round trip OK: demoJob ran")
	case <-time.After(15 * time.Second):
		log.Errorf(ctx, log.TagAppDef, "HANDLER timed out")
		os.Exit(1)
	}

	// Prove the kill fix: trigger job 2 with a DIFFERENT logId (2002), then
	// kill and idleBeat by jobId — pre-fix code keyed the running table by
	// logId, so both would miss.
	postJSON := func(url string, body any) *http.Response {
		b, _ := json.Marshal(body)
		resp, err := http.Post(url, "application/json", bytes.NewReader(b))
		if err != nil {
			log.Errorf(ctx, log.TagAppDef, "%s failed: %v", url, err)
			os.Exit(1)
		}
		return resp
	}

	triggerBody, _ := json.Marshal(starter.TriggerParam{
		JobID: 2, ExecutorHandler: "sleepJob", LogID: 2002,
		ExecutorTimeout: 30, LogDateTime: time.Now().UnixMilli(),
	})
	if tresp, err := http.Post("http://"+adminAddr+"/api/trigger", "application/json", bytes.NewReader(triggerBody)); err != nil {
		log.Errorf(ctx, log.TagAppDef, "TRIGGER sleepJob failed: %v", err)
		os.Exit(1)
	} else {
		_ = tresp.Body.Close()
	}

	// idleBeat by jobId=2 must report "job running" (code 500).
	idleResp := postJSON("http://127.0.0.1:9999/idleBeat", starter.IdleBeatParam{JobID: 2})
	var idle starter.TriggerResponse
	_ = json.NewDecoder(idleResp.Body).Decode(&idle)
	_ = idleResp.Body.Close()
	if idle.Code != 500 {
		log.Errorf(ctx, log.TagAppDef, "IDLEBEAT expected 500 (running), got %d", idle.Code)
		os.Exit(1)
	}

	// kill by jobId=2 must cancel the running task.
	killResp := postJSON("http://127.0.0.1:9999/kill", starter.KillParam{JobID: 2})
	var kill starter.TriggerResponse
	_ = json.NewDecoder(killResp.Body).Decode(&kill)
	_ = killResp.Body.Close()
	if kill.Code != 200 {
		log.Errorf(ctx, log.TagAppDef, "KILL expected 200, got %d", kill.Code)
		os.Exit(1)
	}

	select {
	case <-sleepDone:
		fmt.Println("kill round trip OK: sleepJob (jobId=2, logId=2002) cancelled")
	case <-time.After(15 * time.Second):
		log.Errorf(ctx, log.TagAppDef, "KILL did not cancel sleepJob")
		os.Exit(1)
	}

	// The completion callback must carry the official HandleCallbackParam
	// shape: a JSON array with logId/logDateTime/handleCode/handleMsg. The
	// killed sleepJob must report logId=2002 with handleCode=500.
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		callbacks.Lock()
		bodies := append([][]byte(nil), callbacks.bodies...)
		callbacks.Unlock()
		for _, body := range bodies {
			var arr []starter.HandleCallbackParam
			if json.Unmarshal(body, &arr) != nil || len(arr) == 0 {
				continue
			}
			for _, c := range arr {
				if c.LogID == 2002 {
					if c.HandleCode != 500 {
						log.Errorf(ctx, log.TagAppDef, "CALLBACK handleCode=%d, want 500 (killed)", c.HandleCode)
						os.Exit(1)
					}
					fmt.Printf("callback shape OK: %+v\n", c)
					syscall.Kill(os.Getpid(), syscall.SIGTERM)
					return
				}
			}
		}
		time.Sleep(100 * time.Millisecond)
	}
	log.Errorf(ctx, log.TagAppDef, "CALLBACK for logId=2002 not received or wrong shape")
	os.Exit(1)
}

func init() {
	var execDir string
	_, filename, _, ok := runtime.Caller(0)
	if ok {
		execDir = filepath.Dir(filename)
	}
	if err := os.Chdir(execDir); err != nil {
		panic(err)
	}
	workDir, _ := os.Getwd()
	fmt.Println(workDir)
}
