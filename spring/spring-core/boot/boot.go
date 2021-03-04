package boot

import (
	"context"

	"github.com/go-spring/spring-core/arg"
	"github.com/go-spring/spring-core/bean"
	"github.com/go-spring/spring-core/core"
	"github.com/go-spring/spring-core/mq"
)

var gApp = NewApplication()

func ExpectSysProperties(pattern ...string) {
	gApp.expectSysProperties = pattern
}

// Profile 设置运行环境
func Profile(profile string) {
	gApp.appCtx.SetProfile(profile)
}

// GetProfile 返回运行环境
func GetProfile() string {
	return gApp.appCtx.GetProfile()
}

func GetProperty(key string) interface{} {
	return gApp.appCtx.GetProperty(key)
}

func WireBean(i interface{}) error {
	return gApp.appCtx.WireBean(i)
}

func GetBean(i interface{}, selector ...bean.Selector) bool {
	return gApp.appCtx.GetBean(i, selector...)
}

func FindBean(selector bean.Selector) (bean.Definition, bool) {
	return gApp.appCtx.FindBean(selector)
}

func CollectBeans(i interface{}, selectors ...bean.Selector) bool {
	return gApp.appCtx.CollectBeans(i, selectors...)
}

func Invoke(fn interface{}, args ...arg.Arg) error {
	return gApp.appCtx.Invoke(fn, args...)
}

type GoFuncWithContext func(context.Context)

func Go(fn GoFuncWithContext) {
	appCtx := gApp.appCtx
	appCtx.Go(func() { fn(appCtx.Context()) })
}

func ObjBean(i interface{}) *core.BeanDefinition {
	return gApp.appCtx.ObjBean(i)
}

func CtorBean(fn interface{}, args ...arg.Arg) *core.BeanDefinition {
	return gApp.appCtx.CtorBean(fn, args...)
}

func Config(fn interface{}, args ...arg.Arg) *core.Configer {
	return gApp.appCtx.Config(fn, args...)
}

type gRpcServer struct {

	// Fn 服务注册函数
	Fn interface{}

	// Server 服务提供者
	Server interface{}

	// ServiceName 服务名称
	ServiceName string
}

// GRpcServer 注册 gRPC 服务提供者，fn 是 gRPC 自动生成的服务注册函数，serviceName 是服务名称，
// 必须对应 *_grpc.pg.go 文件里面 grpc.ServiceDesc 的 ServiceName 字段，server 是服务具体提供者对象。
func GRpcServer(serviceName string, fn interface{}, server interface{}) {
	s := &gRpcServer{Fn: fn, Server: server, ServiceName: serviceName}
	gApp.GRpcServers[s.Fn] = s
}

// GRpcClient 注册 gRPC 服务客户端，fn 是 gRPC 自动生成的客户端构造函数
func GRpcClient(fn interface{}, endpoint string) *core.BeanDefinition {
	return gApp.appCtx.CtorBean(fn, endpoint)
}

// Consume 注册 BIND 形式的消息消费者
func Consume(topic string, fn interface{}) {
	gApp.Consumers[topic] = mq.BIND(topic, fn)
}
