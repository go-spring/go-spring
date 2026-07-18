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
	"syscall"
	"time"

	"github.com/casbin/casbin/v2"
	"go-spring.org/spring/gs"

	_ "go-spring.org/starter-casbin"
)

// Service consumes the Casbin enforcer purely by injection. The bean is named
// after its config group key (${spring.casbin.rbac.*} -> "rbac"), so we bind it
// with `autowire:"rbac"`.
type Service struct {
	Enforcer *casbin.Enforcer `autowire:"rbac"`
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

func main() {

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
	// unknown subject -> denied.
	if s.Allowed("carol", "/data", "read") {
		fmt.Fprintln(os.Stderr, "expected carol read denied")
		os.Exit(1)
	}

	fmt.Println("Response from server: casbin rbac enforced ok")
	syscall.Kill(os.Getpid(), syscall.SIGTERM)
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
