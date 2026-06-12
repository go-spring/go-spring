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
	"errors"
	"net"
	"net/http"

	"github.com/gin-gonic/gin"
	"go-spring.org/spring/gs"
	"github.com/go-spring/stdlib/errutil"
)

func init() {
	gs.Provide(
		NewSimpleGinServer,
		gs.IndexArg(1, gs.TagArg("${spring.gin.server}")),
	).Export(gs.As[gs.Server]())
}

type SimpleGinServer struct {
	svr *http.Server
}

func NewSimpleGinServer(e *gin.Engine, cfg gs.SimpleHttpServerConfig) *SimpleGinServer {
	return &SimpleGinServer{svr: &http.Server{
		Handler:           e,
		Addr:              cfg.Address,
		ReadTimeout:       cfg.ReadTimeout,
		ReadHeaderTimeout: cfg.HeaderTimeout,
		WriteTimeout:      cfg.WriteTimeout,
		IdleTimeout:       cfg.IdleTimeout,
	}}
}

func (s *SimpleGinServer) Run(ctx context.Context, sig gs.ReadySignal) error {
	ln, err := net.Listen("tcp", s.svr.Addr)
	if err != nil {
		return err
	}
	<-sig.TriggerAndWait()
	err = s.svr.Serve(ln)
	if errors.Is(err, http.ErrServerClosed) {
		return nil
	}
	return errutil.Explain(err, "failed to serve on %s", s.svr.Addr)
}

func (s *SimpleGinServer) Stop() error {
	return s.svr.Shutdown(context.Background())
}
