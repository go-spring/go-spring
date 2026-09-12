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

// Package main is the example for starter-config-apollo. It starts a mock
// Apollo config service (enough of the meta + config protocol for agollo to
// fetch a namespace), imports the starter, and asserts the remote property
// cold-loads into a Dync field. No docker, no real Apollo stack — the cold-
// load path is what the starter guarantees; hot reload is the same seam and
// is exercised by the unit test.
package main

import (
	"context"
	"flag"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"syscall"
	"time"

	"go-spring.org/log"
	"go-spring.org/spring/gs"
	_ "go-spring.org/starter-config-apollo"
)

const mockAddr = "127.0.0.1:18080"

// Demo holds a Dync property loaded from Apollo.
type Demo struct {
	Message gs.Dync[string] `value:"${demo.message:=none}"`
}

var manual = flag.Bool("manual", false, "run in manual verification mode (server stays up)")

func main() {
	flag.Parse()

	go mockConfigService()

	svrBean := gs.Provide(&Demo{}).Export(gs.As[gs.Rooter]())

	if !*manual {
		go func() {
			time.Sleep(1 * time.Second)
			runTest(svrBean.Interface().(*Demo))
		}()
	} else {
		fmt.Println("=== Manual verification mode ===")
		fmt.Println("Server is running. Follow the README commands in another terminal.")
		fmt.Println("Press Ctrl+C to stop.")
	}
	gs.Run()
}

// mockConfigService serves the two endpoints agollo needs for a cold load:
// /services/config (meta service discovery) and /configs/{appId}/{cluster}/{ns}
// (the namespace content).
func mockConfigService() {
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(os.Stderr, "[mock-apollo] %s %s\n", r.Method, r.URL.String())
		switch r.URL.Path {
		case "/services/config":
			fmt.Fprintf(w, `[{"appName":"demo","instanceId":"mock","homepageUrl":"http://%s"}]`, mockAddr)
		case "/configfiles/json/demo/default/application":
			// The /configfiles/json/... endpoint returns the raw JSON object
			// (unmarshalled straight into the configurations map), not the
			// ApolloConfig envelope.
			fmt.Fprint(w, `{"demo.message":"hello-from-apollo"}`)
		case "/notifications/v2":
			w.WriteHeader(http.StatusNotModified) // no pending change
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	})
	_ = http.ListenAndServe(mockAddr, mux)
}

func runTest(d *Demo) {
	ctx := context.Background()
	got := d.Message.Value()
	if got != "hello-from-apollo" {
		log.Errorf(ctx, log.TagAppDef, "CONFIG mismatch: got %q", got)
		os.Exit(1)
	}
	fmt.Println("Apollo cold-load OK:", got)
	syscall.Kill(os.Getpid(), syscall.SIGTERM)
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
