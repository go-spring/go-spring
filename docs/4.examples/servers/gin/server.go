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
	"net"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/go-spring/spring-core/gs"
)

func init() {
	gs.Provide(
		NewSimpleGinServer,
		gs.IndexArg(1, gs.BindArg(gs.SetHttpServerAddr, gs.TagArg("${http.server.addr:=0.0.0.0:9090}"))),
		gs.IndexArg(1, gs.BindArg(gs.SetHttpServerReadTimeout, gs.TagArg("${http.server.readTimeout:=5s}"))),
		gs.IndexArg(1, gs.BindArg(gs.SetHttpServerHeaderTimeout, gs.TagArg("${http.server.headerTimeout:=1s}"))),
		gs.IndexArg(1, gs.BindArg(gs.SetHttpServerWriteTimeout, gs.TagArg("${http.server.writeTimeout:=5s}"))),
		gs.IndexArg(1, gs.BindArg(gs.SetHttpServerIdleTimeout, gs.TagArg("${http.server.idleTimeout:=60s}"))),
	).AsServer()
}

type SimpleGinServer struct {
	svr *http.Server
}

func NewSimpleGinServer(e *gin.Engine, opts ...gs.HttpServerOption) *SimpleGinServer {
	arg := &gs.HttpServerConfig{
		Address:       "0.0.0.0:9090",
		ReadTimeout:   time.Second * 5,
		HeaderTimeout: time.Second,
		WriteTimeout:  time.Second * 5,
		IdleTimeout:   time.Second * 60,
	}
	for _, opt := range opts {
		opt(arg)
	}
	return &SimpleGinServer{svr: &http.Server{
		Handler:           e,
		Addr:              arg.Address,
		ReadTimeout:       arg.ReadTimeout,
		ReadHeaderTimeout: arg.HeaderTimeout,
		WriteTimeout:      arg.WriteTimeout,
		IdleTimeout:       arg.IdleTimeout,
	}}
}

func (s *SimpleGinServer) ListenAndServe(sig gs.ReadySignal) error {
	ln, err := net.Listen("tcp", s.svr.Addr)
	if err != nil {
		return err
	}
	<-sig.TriggerAndWait()
	return s.svr.Serve(ln)
}

func (s *SimpleGinServer) Shutdown(ctx context.Context) error {
	return s.svr.Shutdown(ctx)
}
