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

package main

import (
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"sync"
	"syscall"
	"time"

	fileadapter "github.com/casbin/casbin/v2/persist/file-adapter"
	"go-spring.org/spring/gs"

	StarterCasbin "go-spring.org/starter-casbin"
)

// Service consumes the Casbin enforcer purely by injection. The bean is named
// after its config group key (${spring.casbin.rbac.*} -> "rbac"), so we bind it
// with `autowire:"rbac"`. The bean type is the starter's *Enforcer wrapper,
// which embeds *casbin.Enforcer, so Enforce and friends are used directly.
type Service struct {
	Enforcer *StarterCasbin.Enforcer `autowire:"rbac"`
}

// Allowed answers whether subject sub may perform act on obj.
func (s *Service) Allowed(sub, obj, act string) bool {
	ok, err := s.Enforcer.Enforce(sub, obj, act)
	if err != nil {
		fmt.Fprintln(os.Stderr, "enforce failed:", err)
		os.Exit(1)
	}
	return ok
}

// policyPath is a runtime-owned copy of the policy, used to prove hot reload
// without mutating the checked-in fixture.
var policyPath string

// watcher is the demo persist.Watcher registered under "local". A production
// deployment would register a distributed watcher (Redis, etcd, ...) whose
// remote updates fire the callback; here we invoke it directly to prove the
// enforcer reloads its policy on signal.
var watcher = &localWatcher{}

// localWatcher is a minimal persist.Watcher: it records the enforcer's reload
// callback and lets the example trigger it via Update, standing in for a
// cross-instance change notification.
type localWatcher struct {
	mu sync.Mutex
	cb func(string)
}

func (w *localWatcher) SetUpdateCallback(cb func(string)) error {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.cb = cb
	return nil
}

func (w *localWatcher) Update() error {
	w.mu.Lock()
	cb := w.cb
	w.mu.Unlock()
	if cb != nil {
		cb("")
	}
	return nil
}

func (w *localWatcher) Close() {}

func main() {
	// Seed a writable policy file from the checked-in fixture and register the
	// adapter + watcher the config points at, before the container starts.
	seedPolicy()
	StarterCasbin.RegisterAdapter("file", fileadapter.NewAdapter(policyPath))
	StarterCasbin.RegisterWatcher("local", watcher)

	// Here `s` is not referenced by any other object,
	// so we need to register it as a root object.
	svrBean := gs.Provide(&Service{}).Export(gs.As[gs.Rooter]())

	http.HandleFunc("/enforce", func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		s := svrBean.Interface().(*Service)
		if s.Allowed(q.Get("sub"), q.Get("obj"), q.Get("act")) {
			_, _ = w.Write([]byte("allow"))
			return
		}
		_, _ = w.Write([]byte("deny"))
	})

	go func() {
		time.Sleep(time.Millisecond * 500)
		runTest(svrBean.Interface().(*Service))
	}()

	// Run the Go-Spring application.
	gs.Run()

	// Example usage:
	//
	// ~ curl "http://127.0.0.1:9090/enforce?sub=alice&obj=/data&act=write"
	// allow
	// ~ curl "http://127.0.0.1:9090/enforce?sub=bob&obj=/data&act=write"
	// deny
}

func runTest(s *Service) {
	// Policies loaded through the registered file adapter.
	// alice is admin -> read+write allowed.
	if !s.Allowed("alice", "/data", "read") {
		fmt.Fprintln(os.Stderr, "expected alice read allowed")
		os.Exit(1)
	}
	if !s.Allowed("alice", "/data", "write") {
		fmt.Fprintln(os.Stderr, "expected alice write allowed")
		os.Exit(1)
	}
	// bob is viewer -> read only.
	if !s.Allowed("bob", "/data", "read") {
		fmt.Fprintln(os.Stderr, "expected bob read allowed")
		os.Exit(1)
	}
	if s.Allowed("bob", "/data", "write") {
		fmt.Fprintln(os.Stderr, "expected bob write denied")
		os.Exit(1)
	}
	// carol is unknown -> denied for now.
	if s.Allowed("carol", "/data", "read") {
		fmt.Fprintln(os.Stderr, "expected carol read denied")
		os.Exit(1)
	}

	// Hot reload: grant carol the admin role in the backing store, then signal
	// the watcher. The enforcer's callback reloads the policy, so carol is now
	// allowed — without restarting the app.
	appendPolicy("g, carol, admin")
	if err := watcher.Update(); err != nil {
		fmt.Fprintln(os.Stderr, "watcher update failed:", err)
		os.Exit(1)
	}
	if !s.Allowed("carol", "/data", "read") {
		fmt.Fprintln(os.Stderr, "expected carol read allowed after hot reload")
		os.Exit(1)
	}

	fmt.Println("Response from server: casbin rbac enforced ok, hot reload applied")
	syscall.Kill(os.Getpid(), syscall.SIGTERM)
}

// seedPolicy copies the checked-in policy into a temp file the example can
// mutate at runtime, leaving the fixture untouched.
func seedPolicy() {
	src, err := os.ReadFile("./conf/policy.csv")
	if err != nil {
		panic(err)
	}
	f, err := os.CreateTemp("", "casbin-policy-*.csv")
	if err != nil {
		panic(err)
	}
	if _, err := f.Write(src); err != nil {
		panic(err)
	}
	_ = f.Close()
	policyPath = f.Name()
}

// appendPolicy adds a policy line to the backing store used by the adapter.
func appendPolicy(line string) {
	f, err := os.OpenFile(policyPath, os.O_APPEND|os.O_WRONLY, 0o600)
	if err != nil {
		fmt.Fprintln(os.Stderr, "append policy failed:", err)
		os.Exit(1)
	}
	defer func() { _ = f.Close() }()
	if _, err := fmt.Fprintln(f, line); err != nil {
		fmt.Fprintln(os.Stderr, "append policy failed:", err)
		os.Exit(1)
	}
}

// ----------------------------------------------------------------------------
// Change working directory
// ----------------------------------------------------------------------------

// init sets the working directory of the application to the directory
// where this source file resides.
// This ensures that any relative file operations are based on the source file location,
// not the process launch path.
func init() {
	var execDir string
	_, filename, _, ok := runtime.Caller(0)
	if ok {
		execDir = filepath.Dir(filename)
	}
	err := os.Chdir(execDir)
	if err != nil {
		panic(err)
	}
	workDir, err := os.Getwd()
	if err != nil {
		panic(err)
	}
	fmt.Println(workDir)
}
