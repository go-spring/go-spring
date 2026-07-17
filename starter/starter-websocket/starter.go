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

package StarterWebsocket

import (
	"time"

	"github.com/gorilla/websocket"
	"go-spring.org/spring/gs"
)

func init() {
	// Contribute a configured *websocket.Upgrader so any HTTP handler
	// (net/http, gin, echo, hertz, ...) can promote a request into a
	// WebSocket connection. This starter provides the upgrade capability
	// only; it does not own an HTTP server or a listening port. Mount your
	// WebSocket routes on whatever HTTP server the application already runs.
	//
	// OnMissingBean lets an application override the default by providing
	// its own *websocket.Upgrader (e.g. with a custom CheckOrigin or
	// compression settings).
	gs.Provide(NewUpgrader, gs.TagArg("${spring.websocket}")).
		Condition(gs.OnMissingBean[*websocket.Upgrader]())
}

// Config tunes the *websocket.Upgrader. It carries no server address on
// purpose: the upgrader is mounted onto an existing HTTP server, which owns
// the listening address and timeouts.
type Config struct {
	HandshakeTimeout time.Duration `value:"${handshakeTimeout:=10s}"`
	ReadBufferSize   int           `value:"${readBufferSize:=1024}"`
	WriteBufferSize  int           `value:"${writeBufferSize:=1024}"`
}

// NewUpgrader builds a *websocket.Upgrader from ${spring.websocket} configuration.
func NewUpgrader(cfg Config) *websocket.Upgrader {
	return &websocket.Upgrader{
		HandshakeTimeout: cfg.HandshakeTimeout,
		ReadBufferSize:   cfg.ReadBufferSize,
		WriteBufferSize:  cfg.WriteBufferSize,
	}
}
