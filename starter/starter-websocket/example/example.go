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
			mux.HandleFunc("/echo", func(w http.ResponseWriter, r *http.Request) {
				conn, err := upgrader.Upgrade(w, r, nil)
				if err != nil {
					log.Errorf(r.Context(), log.TagAppDef, "Failed to upgrade: %v", err)
					return
				}
				c.Echo(conn)
			})
		}
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

func main() {
	_ = os.Unsetenv("_")
	_ = os.Unsetenv("TERM")
	_ = os.Unsetenv("TERM_SESSION_ID")

	go func() {
		time.Sleep(time.Millisecond * 500)
		runTest()
	}()

	gs.Configure(func(app gs.App) {
		app.Property("spring.http.server.enabled", "false")
	}).Run()
}

func runTest() {
	conn, _, err := websocket.DefaultDialer.Dial("ws://127.0.0.1:9696/echo", nil)
	if err != nil {
		log.Errorf(context.Background(), log.TagAppDef, "Failed to connect: %v", err)
		os.Exit(1)
	}
	defer conn.Close()

	if err = conn.WriteMessage(websocket.TextMessage, []byte("Hello, WebSocket!")); err != nil {
		log.Errorf(context.Background(), log.TagAppDef, "Error sending message: %v", err)
		os.Exit(1)
	}

	_, msg, err := conn.ReadMessage()
	if err != nil {
		log.Errorf(context.Background(), log.TagAppDef, "Error reading message: %v", err)
		os.Exit(1)
	}
	fmt.Println("Response from server:", string(msg))
	if string(msg) != "Hello, WebSocket!" {
		os.Exit(1)
	}

	syscall.Kill(os.Getpid(), syscall.SIGTERM)
}
