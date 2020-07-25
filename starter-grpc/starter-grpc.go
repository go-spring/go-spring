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

package StarterGrpc

import (
	"fmt"
	"net"
	"reflect"
	"runtime"
	"strings"

	"github.com/go-spring/go-spring-parent/spring-logger"
	"github.com/go-spring/go-spring-parent/spring-utils"
	"github.com/go-spring/go-spring/spring-boot"
	"google.golang.org/grpc"
)

func init() {
	SpringBoot.RegisterBeanFn(NewGRpcServerStarter)
}

// GRpcServerConfig gRPC 服务器配置
type GRpcServerConfig struct {
	Port int `value:"${grpc.server.port:=9090}"` // gRPC 端口
}

// GRpcServerStarter gRPC 服务器启动器
type GRpcServerStarter struct {
	_ SpringBoot.ApplicationEvent `export:""`

	Config GRpcServerConfig `value:"${}"`
	server *grpc.Server
}

// NewGRpcServerStarter GRpcServerStarter 的构造函数
func NewGRpcServerStarter() *GRpcServerStarter {
	return &GRpcServerStarter{
		server: grpc.NewServer(),
	}
}

func (starter *GRpcServerStarter) OnStartApplication(ctx SpringBoot.ApplicationContext) {

	addr := fmt.Sprintf(":%d", starter.Config.Port)
	lis, err := net.Listen("tcp", addr)
	SpringUtils.Panic(err).When(err != nil)

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
			SpringLogger.Infof("/%s/%s %s:%d", service, method.Name, file, line)
		}
	}

	ctx.SafeGoroutine(func() {
		if err = starter.server.Serve(lis); err != nil {
			SpringLogger.Error(err)
		}
	})
}

func (starter *GRpcServerStarter) OnStopApplication(ctx SpringBoot.ApplicationContext) {
	starter.server.GracefulStop()
}
