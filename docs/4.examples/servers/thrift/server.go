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
	"time"

	"github.com/apache/thrift/lib/go/thrift"
	"github.com/go-spring/spring-core/gs"
)

func init() {
	gs.Object(&SimpleThriftServer{}).AsServer().Condition(
		gs.OnBean[thrift.TProcessor](),
	)
}

type SimpleThriftServer struct {
	Addr string            `value:"${thrift.server.addr:=0.0.0.0:9292}"`
	Proc thrift.TProcessor `autowire:""`
	svr  *thrift.TSimpleServer
}

func (s *SimpleThriftServer) ListenAndServe(sig gs.ReadySignal) error {
	transport, err := thrift.NewTServerSocket(s.Addr)
	if err != nil {
		return err
	}
	s.svr = thrift.NewTSimpleServer2(s.Proc, transport)
	<-sig.TriggerAndWait()
	return s.svr.Serve()
}

func (s *SimpleThriftServer) Shutdown(ctx context.Context) error {
	thrift.ServerStopTimeout = time.Second
	return s.svr.Stop()
}
