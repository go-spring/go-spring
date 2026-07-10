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

	"github.com/gorilla/websocket"
	"go-spring.org/log"
	"go-spring.org/spring/gs"
	StarterWebsocket "go-spring.org/starter-websocket"
)

func init() {
	gs.Provide(&Controller{})
	gs.Provide(func(c *Controller) StarterWebsocket.ServerRegister {
		return func(mux *http.ServeMux, upgrader *websocket.Upgrader) {
			// Route 1: plain text echo, guarded by the X-App middleware.
			mux.Handle("/echo", requireApp(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				conn, err := upgrader.Upgrade(w, r, nil)
				if err != nil {
					log.Errorf(r.Context(), log.TagAppDef, "Failed to upgrade /echo: %v", err)
					return
				}
				c.Echo(conn)
			})))

			// Route 2: JSON message echo, also guarded by the X-App middleware.
			mux.Handle("/json", requireApp(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				conn, err := upgrader.Upgrade(w, r, nil)
				if err != nil {
					log.Errorf(r.Context(), log.TagAppDef, "Failed to upgrade /json: %v", err)
					return
				}
				c.EchoJSON(conn)
			})))

			// Route 3: guarded route used to demonstrate that the middleware
			// rejects requests missing the X-App header before any upgrade.
			mux.Handle("/guard", requireApp(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				conn, err := upgrader.Upgrade(w, r, nil)
				if err != nil {
					log.Errorf(r.Context(), log.TagAppDef, "Failed to upgrade /guard: %v", err)
					return
				}
				c.Echo(conn)
			})))
		}
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
func (c *Controller) Echo(conn *websocket.Conn) {
	defer conn.Close()
	for {
		mt, msg, err := conn.ReadMessage()
		if err != nil {
			return
		}
		if err = conn.WriteMessage(mt, msg); err != nil {
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
func (c *Controller) EchoJSON(conn *websocket.Conn) {
	defer conn.Close()
	for {
		var req EchoRequest
		if err := conn.ReadJSON(&req); err != nil {
			return
		}
		resp := EchoResponse{Message: "Hi, " + req.Name}
		if err := conn.WriteJSON(&resp); err != nil {
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
	echoConn, _, err := websocket.DefaultDialer.Dial("ws://127.0.0.1:9696/echo", authHeader)
	if err != nil {
		log.Errorf(ctx, log.TagAppDef, "Failed to connect /echo: %v", err)
		os.Exit(1)
	}
	defer echoConn.Close()

	if err = echoConn.WriteMessage(websocket.TextMessage, []byte("Hello, WebSocket!")); err != nil {
		log.Errorf(ctx, log.TagAppDef, "Error sending text message: %v", err)
		os.Exit(1)
	}
	_, msg, err := echoConn.ReadMessage()
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
	jsonConn, _, err := websocket.DefaultDialer.Dial("ws://127.0.0.1:9696/json", authHeader)
	if err != nil {
		log.Errorf(ctx, log.TagAppDef, "Failed to connect /json: %v", err)
		os.Exit(1)
	}
	defer jsonConn.Close()

	if err = jsonConn.WriteJSON(EchoRequest{Name: "world"}); err != nil {
		log.Errorf(ctx, log.TagAppDef, "Error sending JSON: %v", err)
		os.Exit(1)
	}
	var resp EchoResponse
	if err = jsonConn.ReadJSON(&resp); err != nil {
		log.Errorf(ctx, log.TagAppDef, "Error reading JSON: %v", err)
		os.Exit(1)
	}
	fmt.Println("Response from server:", resp.Message)
	if resp.Message != "Hi, world" {
		log.Errorf(ctx, log.TagAppDef, "Unexpected JSON echo: %q", resp.Message)
		os.Exit(1)
	}

	// Feature 3a: middleware allows /guard when header is present.
	guardConn, _, err := websocket.DefaultDialer.Dial("ws://127.0.0.1:9696/guard", authHeader)
	if err != nil {
		log.Errorf(ctx, log.TagAppDef, "Failed to connect /guard with header: %v", err)
		os.Exit(1)
	}
	guardConn.Close()
	fmt.Println("Response from server: /guard accepted request with X-App header")

	// Feature 3b: middleware rejects /guard when header is missing.
	badConn, badResp, err := websocket.DefaultDialer.Dial("ws://127.0.0.1:9696/guard", nil)
	if err == nil {
		badConn.Close()
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
