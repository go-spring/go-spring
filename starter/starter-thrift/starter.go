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

package StarterThrift

import (
	"context"

	"github.com/apache/thrift/lib/go/thrift"
	"go-spring.org/spring/gs"
	"go-spring.org/stdlib/errutil"
	"go-spring.org/stdlib/flatten"
)

func init() {
	enableSimpleThriftServer := gs.OnProperty("spring.thrift.server.enabled").
		HavingValue("true").MatchIfMissing()
	gs.Module(enableSimpleThriftServer, func(r gs.BeanProvider, p flatten.Storage) error {

		// Register the Thrift server
		// when a processor is available.
		r.Provide(
			NewSimpleThriftServer,
			gs.IndexArg(0, gs.TagArg("${spring.thrift.server}")),
		).Export(gs.As[gs.Server]()).
			Condition(gs.OnBean[thrift.TProcessor]())
		return nil
	})
}

// Config defines Thrift server configuration.
type Config struct {
	Addr string `value:"${addr:=:9292}"`
}

// SimpleThriftServer adapts a thrift.TSimpleServer to the Go-Spring server lifecycle.
type SimpleThriftServer struct {
	cfg  Config
	proc thrift.TProcessor
	svr  *thrift.TSimpleServer
}

// NewSimpleThriftServer creates a SimpleThriftServer from ${spring.thrift.server} configuration.
func NewSimpleThriftServer(cfg Config, proc thrift.TProcessor) *SimpleThriftServer {
	return &SimpleThriftServer{cfg: cfg, proc: proc}
}

// Run starts the Thrift server after Go-Spring signals readiness.
func (s *SimpleThriftServer) Run(ctx context.Context, sig gs.ReadySignal) error {
	transport, err := thrift.NewTServerSocket(s.cfg.Addr)
	if err != nil {
		return errutil.Explain(err, "failed to listen on %s", s.cfg.Addr)
	}
	s.svr = thrift.NewTSimpleServer2(s.proc, transport)
	<-sig.TriggerAndWait()
	if err = s.svr.Serve(); err != nil {
		return errutil.Explain(err, "failed to serve on %s", s.cfg.Addr)
	}
	return nil
}

// Stop gracefully stops the underlying Thrift server.
func (s *SimpleThriftServer) Stop() error {
	return s.svr.Stop()
}
