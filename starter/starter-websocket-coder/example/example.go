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
	"context"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"syscall"
	"time"

	"github.com/coder/websocket"
	"github.com/coder/websocket/wsjson"
	"go-spring.org/log"
	"go-spring.org/spring/gs"
	_ "go-spring.org/starter-websocket-coder"
)

func init() {
	gs.Provide(&Controller{})

	// The websocket starter no longer owns an HTTP server: it only contributes
	// a configured *websocket.AcceptOptions. We mount the WebSocket routes onto
	// the HTTP server that Go-Spring already runs by providing a custom
	// *gs.HttpServeMux (which overrides the default http.DefaultServeMux).
	gs.Provide(func(c *Controller, opts *websocket.AcceptOptions) *gs.HttpServeMux {
		mux := http.NewServeMux()

		// Route 1: plain text echo, guarded by the X-App middleware.
		mux.Handle("/echo", requireApp(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			conn, err := websocket.Accept(w, r, opts)
			if err != nil {
				log.Errorf(r.Context(), log.TagAppDef, "Failed to upgrade /echo: %v", err)
				return
			}
			c.Echo(r.Context(), conn)
		})))

		// Route 2: JSON message echo, also guarded by the X-App middleware.
		mux.Handle("/json", requireApp(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			conn, err := websocket.Accept(w, r, opts)
			if err != nil {
				log.Errorf(r.Context(), log.TagAppDef, "Failed to upgrade /json: %v", err)
				return
			}
			c.EchoJSON(r.Context(), conn)
		})))

		// Route 3: guarded route used to demonstrate that the middleware
		// rejects requests missing the X-App header before any upgrade.
		mux.Handle("/guard", requireApp(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			conn, err := websocket.Accept(w, r, opts)
			if err != nil {
				log.Errorf(r.Context(), log.TagAppDef, "Failed to upgrade /guard: %v", err)
				return
			}
			c.Echo(r.Context(), conn)
		})))

		return &gs.HttpServeMux{Handler: mux}
	})
}

// requireApp is an HTTP middleware that only lets requests carrying the
// header `X-App: go-spring` reach the upgrade handler. Everything else
// gets a 403 before the WebSocket handshake even starts.
func requireApp(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("X-App") != "go-spring" {
			http.Error(w, "forbidden: missing or invalid X-App header", http.StatusForbidden)
			return
		}
		next.ServeHTTP(w, r)
	})
}

type Controller struct{}

// Echo reads each message from the connection and writes it back unchanged
// until the peer closes the connection or an error occurs.
func (c *Controller) Echo(ctx context.Context, conn *websocket.Conn) {
	defer conn.CloseNow()
	for {
		mt, msg, err := conn.Read(ctx)
		if err != nil {
			return
		}
		if err = conn.Write(ctx, mt, msg); err != nil {
			return
		}
	}
}

// EchoRequest is the payload the /json route accepts.
type EchoRequest struct {
	Name string `json:"name"`
}

// EchoResponse is the payload the /json route emits.
type EchoResponse struct {
	Message string `json:"message"`
}

// EchoJSON reads a JSON EchoRequest from the connection and writes back a
// JSON EchoResponse greeting the requested name.
func (c *Controller) EchoJSON(ctx context.Context, conn *websocket.Conn) {
	defer conn.CloseNow()
	for {
		var req EchoRequest
		if err := wsjson.Read(ctx, conn, &req); err != nil {
			return
		}
		resp := EchoResponse{Message: "Hi, " + req.Name}
		if err := wsjson.Write(ctx, conn, &resp); err != nil {
			return
		}
	}
}

func main() {
	_ = os.Unsetenv("_")
	_ = os.Unsetenv("TERM")
	_ = os.Unsetenv("TERM_SESSION_ID")

	go func() {
		time.Sleep(time.Millisecond * 500)
		runTest()
	}()

	gs.Run()
}

func runTest() {
	ctx := context.Background()
	authHeader := http.Header{"X-App": []string{"go-spring"}}

	// Feature 1: text echo on /echo (middleware allowed).
	echoConn, _, err := websocket.Dial(ctx, "ws://127.0.0.1:9797/echo", &websocket.DialOptions{HTTPHeader: authHeader})
	if err != nil {
		log.Errorf(ctx, log.TagAppDef, "Failed to connect /echo: %v", err)
		os.Exit(1)
	}
	defer echoConn.CloseNow()

	if err = echoConn.Write(ctx, websocket.MessageText, []byte("Hello, WebSocket!")); err != nil {
		log.Errorf(ctx, log.TagAppDef, "Error sending text message: %v", err)
		os.Exit(1)
	}
	_, msg, err := echoConn.Read(ctx)
	if err != nil {
		log.Errorf(ctx, log.TagAppDef, "Error reading text message: %v", err)
		os.Exit(1)
	}
	fmt.Println("Response from server:", string(msg))
	if string(msg) != "Hello, WebSocket!" {
		log.Errorf(ctx, log.TagAppDef, "Unexpected text echo: %q", string(msg))
		os.Exit(1)
	}

	// Feature 2: JSON echo on /json.
	jsonConn, _, err := websocket.Dial(ctx, "ws://127.0.0.1:9797/json", &websocket.DialOptions{HTTPHeader: authHeader})
	if err != nil {
		log.Errorf(ctx, log.TagAppDef, "Failed to connect /json: %v", err)
		os.Exit(1)
	}
	defer jsonConn.CloseNow()

	if err = wsjson.Write(ctx, jsonConn, EchoRequest{Name: "world"}); err != nil {
		log.Errorf(ctx, log.TagAppDef, "Error sending JSON: %v", err)
		os.Exit(1)
	}
	var resp EchoResponse
	if err = wsjson.Read(ctx, jsonConn, &resp); err != nil {
		log.Errorf(ctx, log.TagAppDef, "Error reading JSON: %v", err)
		os.Exit(1)
	}
	fmt.Println("Response from server:", resp.Message)
	if resp.Message != "Hi, world" {
		log.Errorf(ctx, log.TagAppDef, "Unexpected JSON echo: %q", resp.Message)
		os.Exit(1)
	}

	// Feature 3a: middleware allows /guard when header is present.
	guardConn, _, err := websocket.Dial(ctx, "ws://127.0.0.1:9797/guard", &websocket.DialOptions{HTTPHeader: authHeader})
	if err != nil {
		log.Errorf(ctx, log.TagAppDef, "Failed to connect /guard with header: %v", err)
		os.Exit(1)
	}
	guardConn.CloseNow()
	fmt.Println("Response from server: /guard accepted request with X-App header")

	// Feature 3b: middleware rejects /guard when header is missing.
	badConn, badResp, err := websocket.Dial(ctx, "ws://127.0.0.1:9797/guard", nil)
	if err == nil {
		badConn.CloseNow()
		log.Errorf(ctx, log.TagAppDef, "Expected /guard to reject unauthenticated dial, but it succeeded")
		os.Exit(1)
	}
	if badResp == nil || badResp.StatusCode != http.StatusForbidden {
		got := 0
		if badResp != nil {
			got = badResp.StatusCode
		}
		log.Errorf(ctx, log.TagAppDef, "Expected 403 from /guard without header, got status=%d err=%v", got, err)
		os.Exit(1)
	}
	fmt.Println("Response from server: /guard rejected request without X-App header (status 403)")

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
