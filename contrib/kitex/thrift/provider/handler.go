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

	"github.com/cloudwego/kitex/pkg/klog"
	"github.com/cloudwego/kitex/server"
	echo "go-spring.org/kitex/thrift/idl/echo"
	"go-spring.org/kitex/thrift/idl/echo/echoservice"
	"go-spring.org/spring/gs"
	StarterKitex "go-spring.org/starter-kitex"
)

func init() {
	// Provide a StarterKitex.ServiceRegister bean that binds the EchoServiceImpl
	// to a raw Kitex server via the generated echoservice.RegisterService.
	// starter-kitex's SimpleKitexServer depends only on this function type, so
	// the concrete service is wired here without the server ever knowing about
	// the generated echo service.
	gs.Provide(func() StarterKitex.ServiceRegister {
		return func(svr server.Server) error {
			return echoservice.RegisterService(svr, &EchoServiceImpl{})
		}
	})
}

// EchoServiceImpl implements the EchoService interface defined in echo.thrift.
type EchoServiceImpl struct{}

// Echo returns the request message unchanged, giving the client a
// deterministic value to assert on. The klog.CtxInfof call is context-aware, so
// the obs-opentelemetry logrus adapter (installed by starter-kitex) tags each
// line with the request's trace_id/span_id, correlating logs with traces.
func (s *EchoServiceImpl) Echo(ctx context.Context, req *echo.EchoRequest) (*echo.EchoResponse, error) {
	klog.CtxInfof(ctx, "echo request: %s", req.Message)
	return &echo.EchoResponse{Message: req.Message}, nil
}
