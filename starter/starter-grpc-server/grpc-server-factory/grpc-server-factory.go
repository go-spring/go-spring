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

package GrpcServerFactory

import (
	"fmt"
	"net"
	"reflect"
	"runtime"
	"strings"

	"github.com/go-spring/spring-boot"
	"github.com/go-spring/spring-logger"
	"github.com/go-spring/spring-utils"
	"github.com/go-spring/starter-grpc"
	"google.golang.org/grpc"
)

// Starter gRPC 服务器启动器
type Starter struct {
	_ SpringBoot.ApplicationEvent `export:""`

	config StarterGrpc.ServerConfig
	server *grpc.Server
}

// NewStarter Starter 的构造函数
func NewStarter(config StarterGrpc.ServerConfig) *Starter {
	return &Starter{
		config: config,
		server: grpc.NewServer(),
	}
}

func (starter *Starter) OnStartApplication(ctx SpringBoot.ApplicationContext) {

	srvMap := make(map[string]reflect.Value)

	v := reflect.ValueOf(starter.server)
	for fn, server := range SpringBoot.GRpcServerMap {
		if server.CheckCondition(ctx) {
			ctx.WireBean(server.Server()) // 对 gRPC 服务对象进行注入
			srv := reflect.ValueOf(server.Server())
			fn.Call([]reflect.Value{v, srv}) // 调用 gRPC 的服务注册函数
			tName := strings.TrimSuffix(fn.Type().In(1).String(), "Server")
			srvMap[tName] = srv
		}
	}

	for service, info := range starter.server.GetServiceInfo() {
		srv := srvMap[service]
		for _, method := range info.Methods {
			m, _ := srv.Type().MethodByName(method.Name)
			fnPtr := m.Func.Pointer()
			fnInfo := runtime.FuncForPC(fnPtr)
			file, line := fnInfo.FileLine(fnPtr)
			SpringLogger.Infof("/%s/%s %s:%d ", service, method.Name, file, line)
		}
	}

	addr := fmt.Sprintf(":%d", starter.config.Port)
	lis, err := net.Listen("tcp", addr)
	SpringUtils.Panic(err).When(err != nil)

	ctx.SafeGoroutine(func() {
		if err = starter.server.Serve(lis); err != nil {
			SpringLogger.Error(err)
		}
	})
}

func (starter *Starter) OnStopApplication(ctx SpringBoot.ApplicationContext) {
	starter.server.GracefulStop()
}
