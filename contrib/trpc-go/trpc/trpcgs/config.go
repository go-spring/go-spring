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

package trpcgs

import (
	"trpc.group/trpc-go/trpc-go/server"
)

// ServiceRegister binds a service handler onto a tRPC server.Server. This
// function type keeps SimpleTrpcServer service-agnostic: it drives the
// lifecycle while each service supplies its own register bean, typically
// wrapping the generated xxx.RegisterXxxServiceService.
type ServiceRegister func(s *server.Server)

// Config defines the tRPC server configuration, bound from ${spring.trpc.server}.
//
// tRPC-Go's own trpc.NewServer() reads a trpc_go.yaml and owns configuration +
// plugin bootstrap globally. This example takes the other fork: it builds a tRPC
// *Config programmatically from these Go-Spring properties and hands it to
// trpc.NewServerWithConfig, so there is no trpc_go.yaml and all config lives in
// conf/app.properties like every other Go-Spring service.
type Config struct {
	// Addr is the host:port the service listens on, split into tRPC's IP/Port.
	Addr string `value:"${addr:=127.0.0.1:8000}"`
	// ServiceName is the fully-qualified tRPC service name (trpc.app.server.service).
	// It must match the callee name baked into the generated client stub.
	ServiceName string `value:"${service.name:=trpc.helloworld.greet.GreetService}"`
	// Network / Protocol default to tRPC's own tcp + trpc wire protocol.
	Network  string `value:"${network:=tcp}"`
	Protocol string `value:"${protocol:=trpc}"`
}
