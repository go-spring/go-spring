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

// Package main is the self-contained example for starter-webhook: it starts
// a local HTTP receiver that stands in for a chat-platform webhook, sends a
// notification through the starter, and asserts the payload that arrives on
// the wire. No docker needed — the smoke test is just `go run .`.
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"syscall"
	"time"

	"go-spring.org/spring/gs"

	starter "go-spring.org/starter-webhook"
)

const receiverAddr = "127.0.0.1:18080"

type Service struct {
	Notifier *starter.Notifier `autowire:"local"`
}

var manual = flag.Bool("manual", false, "run in manual verification mode (server stays up)")

func main() {
	flag.Parse()

	// Stand in for the chat platform: capture the POSTed webhook body.
	received := make(chan map[string]any, 1)
	mux := http.NewServeMux()
	mux.HandleFunc("/hook", func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		var m map[string]any
		_ = json.Unmarshal(b, &m)
		select {
		case received <- m:
		default:
		}
		w.WriteHeader(http.StatusOK)
	})
	srv := &http.Server{Addr: receiverAddr, Handler: mux, ReadHeaderTimeout: 5 * time.Second}
	go func() { _ = srv.ListenAndServe() }()

	// Here `s` is not referenced by any other object,
	// so we need to register it as a root object.
	svrBean := gs.Provide(&Service{}).Export(gs.As[gs.Rooter]())

	if !*manual {
		go func() {
			time.Sleep(time.Millisecond * 500)
			runTest(svrBean.Interface().(*Service), received)
		}()
	} else {

		// Run the Go-Spring application.

		fmt.Println("=== Manual verification mode ===")
		fmt.Printf("Receiver listening on http://%s/hook\n", receiverAddr)
		fmt.Println("Press Ctrl+C to stop.")
	}
	gs.Run()
	_ = srv.Close()
}

func runTest(s *Service, received chan map[string]any) {
	if err := s.Notifier.Send(context.Background(), &starter.Notification{Title: "deploy", Text: "example finished"}); err != nil {
		fmt.Fprintln(os.Stderr, "SEND failed:", err)
		os.Exit(1)
	}

	select {
	case m := <-received:
		if m["title"] != "deploy" || m["text"] != "example finished" {
			fmt.Fprintf(os.Stderr, "RECEIVE mismatch: %v\n", m)
			os.Exit(1)
		}
		fmt.Println("Webhook delivered:", m)
	case <-time.After(10 * time.Second):
		fmt.Fprintln(os.Stderr, "RECEIVE failed: timed out waiting for webhook")
		os.Exit(1)
	}

	syscall.Kill(os.Getpid(), syscall.SIGTERM)
}

// ----------------------------------------------------------------------------
// Change working directory
// ----------------------------------------------------------------------------

// init sets the working directory of the application to the directory
// where this source file resides.
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
