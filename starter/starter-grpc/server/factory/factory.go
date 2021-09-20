/*
 * Copyright 2012-2019 the original author or authors.
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

package factory

import (
	"context"
	"fmt"
	"net"
	"reflect"
	"runtime"

	"github.com/go-spring/spring-boost/log"
	"github.com/go-spring/spring-boost/util"
	SpringGrpc "github.com/go-spring/spring-core/grpc"
	"github.com/go-spring/spring-core/gs"
	"github.com/go-spring/starter-core"
	"google.golang.org/grpc"
)

// Starter gRPC 服务器启动器
type Starter struct {
	config  StarterCore.GrpcServerConfig
	server  *grpc.Server
	Servers map[string]*SpringGrpc.Server `autowire:""`
}

// NewStarter Starter 的构造函数
func NewStarter(config StarterCore.GrpcServerConfig) *Starter {
	return &Starter{
		config: config,
		server: grpc.NewServer(),
	}
}

func (starter *Starter) OnStartApp(ctx gs.Environment) {

	server := reflect.ValueOf(starter.server)
	srvMap := make(map[string]reflect.Value)
	for serviceName, rpcServer := range starter.Servers {

		service := reflect.ValueOf(rpcServer.Service)
		srvMap[serviceName] = service

		fn := reflect.ValueOf(rpcServer.Register)
		fn.Call([]reflect.Value{server, service})
	}

	for service, info := range starter.server.GetServiceInfo() {
		srv := srvMap[service]
		for _, method := range info.Methods {
			m, _ := srv.Type().MethodByName(method.Name)
			fnPtr := m.Func.Pointer()
			fnInfo := runtime.FuncForPC(fnPtr)
			file, line := fnInfo.FileLine(fnPtr)
			log.Infof("/%s/%s %s:%d ", service, method.Name, file, line)
		}
	}

	addr := fmt.Sprintf(":%d", starter.config.Port)
	listener, err := net.Listen("tcp", addr)
	util.Panic(err).When(err != nil)

	ctx.Go(func(_ context.Context) {
		if err = starter.server.Serve(listener); err != nil {
			log.Error(err)
		}
	})
}

func (starter *Starter) OnStopApp(ctx context.Context) {
	starter.server.GracefulStop()
}
