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

package StarterWebsocketCoder

import (
	"github.com/coder/websocket"
	"go-spring.org/spring/gs"
)

func init() {
	// Contribute a configured *websocket.AcceptOptions so any HTTP handler
	// (net/http, gin, echo, hertz, ...) can promote a request into a
	// WebSocket connection via websocket.Accept. This starter provides the
	// upgrade options only; it does not own an HTTP server or a listening
	// port. Mount your WebSocket routes on whatever HTTP server the
	// application already runs.
	//
	// Unlike gorilla/websocket, coder/websocket has no Upgrader object: the
	// server upgrade is the free function websocket.Accept(w, r, *AcceptOptions).
	// We therefore contribute the *AcceptOptions itself as the injectable bean.
	//
	// OnMissingBean lets an application override the default by providing its
	// own *websocket.AcceptOptions (e.g. with Subprotocols or a custom
	// CheckOrigin-equivalent via OriginPatterns).
	gs.Provide(NewAcceptOptions, gs.TagArg("${spring.websocket}")).
		Condition(gs.OnMissingBean[*websocket.AcceptOptions]())
}

// Config tunes the *websocket.AcceptOptions. It carries no server address on
// purpose: the options are used to upgrade requests on an existing HTTP
// server, which owns the listening address and timeouts.
type Config struct {
	InsecureSkipVerify   bool     `value:"${insecureSkipVerify:=false}"`
	OriginPatterns       []string `value:"${originPatterns:=}"`
	CompressionMode      int      `value:"${compressionMode:=0}"`
	CompressionThreshold int      `value:"${compressionThreshold:=0}"`
}

// NewAcceptOptions builds a *websocket.AcceptOptions from ${spring.websocket}
// configuration.
func NewAcceptOptions(cfg Config) *websocket.AcceptOptions {
	return &websocket.AcceptOptions{
		InsecureSkipVerify:   cfg.InsecureSkipVerify,
		OriginPatterns:       cfg.OriginPatterns,
		CompressionMode:      websocket.CompressionMode(cfg.CompressionMode),
		CompressionThreshold: cfg.CompressionThreshold,
	}
}
