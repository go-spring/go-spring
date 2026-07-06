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
	"context"
	"errors"
	"net"
	"net/http"
	"time"

	"github.com/gorilla/websocket"
	"go-spring.org/spring/gs"
	"go-spring.org/stdlib/errutil"
	"go-spring.org/stdlib/flatten"
)

func init() {
	enableSimpleWebsocketServer := gs.OnProperty("spring.websocket.server.enabled").
		HavingValue("true").MatchIfMissing()
	gs.Module(enableSimpleWebsocketServer, func(r gs.BeanProvider, p flatten.Storage) error {

		// Register the WebSocket server
		// when a route register is available.
		r.Provide(
			NewSimpleWebsocketServer,
			gs.IndexArg(0, gs.TagArg("${spring.websocket.server}")),
		).Export(gs.As[gs.Server]()).
			Condition(gs.OnBean[ServerRegister]())
		return nil
	})
}

// ServerRegister registers WebSocket routes onto the mux. Use the provided
// upgrader to promote an HTTP request into a WebSocket connection.
type ServerRegister func(mux *http.ServeMux, upgrader *websocket.Upgrader)

// Config defines WebSocket server configuration. The read/write buffer sizes
// and handshake timeout tune the upgrader; the HTTP server itself uses no
// read/write timeout so long-lived connections are not cut off.
type Config struct {
	Addr             string        `value:"${addr:=:9696}"`
	HandshakeTimeout time.Duration `value:"${handshakeTimeout:=10s}"`
	ReadBufferSize   int           `value:"${readBufferSize:=1024}"`
	WriteBufferSize  int           `value:"${writeBufferSize:=1024}"`
}

// SimpleWebsocketServer adapts a WebSocket-upgrading HTTP server to the
// Go-Spring server lifecycle.
type SimpleWebsocketServer struct {
	cfg Config
	reg ServerRegister
	svr *http.Server
}

// NewSimpleWebsocketServer creates a SimpleWebsocketServer from ${spring.websocket.server} configuration.
func NewSimpleWebsocketServer(cfg Config, reg ServerRegister) *SimpleWebsocketServer {
	return &SimpleWebsocketServer{cfg: cfg, reg: reg}
}

// Run starts the WebSocket server after Go-Spring signals readiness.
func (s *SimpleWebsocketServer) Run(ctx context.Context, sig gs.ReadySignal) error {
	upgrader := &websocket.Upgrader{
		HandshakeTimeout: s.cfg.HandshakeTimeout,
		ReadBufferSize:   s.cfg.ReadBufferSize,
		WriteBufferSize:  s.cfg.WriteBufferSize,
	}
	mux := http.NewServeMux()
	s.reg(mux, upgrader)
	s.svr = &http.Server{Addr: s.cfg.Addr, Handler: mux}

	ln, err := net.Listen("tcp", s.cfg.Addr)
	if err != nil {
		return errutil.Explain(err, "failed to listen on %s", s.cfg.Addr)
	}
	<-sig.TriggerAndWait()
	if err = s.svr.Serve(ln); err != nil && !errors.Is(err, http.ErrServerClosed) {
		return errutil.Explain(err, "failed to serve on %s", s.cfg.Addr)
	}
	return nil
}

// Stop gracefully stops the underlying HTTP server.
func (s *SimpleWebsocketServer) Stop() error {
	return s.svr.Shutdown(context.Background())
}
