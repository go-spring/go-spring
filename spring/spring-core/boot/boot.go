package boot

import (
	"context"

	"github.com/go-spring/spring-core/app"
	"github.com/go-spring/spring-core/arg"
	"github.com/go-spring/spring-core/bean"
	"github.com/go-spring/spring-core/core"
	"github.com/go-spring/spring-core/grpc"
	"github.com/go-spring/spring-core/web"
)

var gApp = app.NewApplication()

func App() *app.Application { return gApp }

func ApplicationContext() core.ApplicationContext {
	return gApp.ApplicationContext()
}

// GetProfile 返回运行环境
func GetProfile() string {
	return ApplicationContext().GetProfile()
}

func GetProperty(key string) interface{} {
	return ApplicationContext().GetProperty(key)
}

func WireBean(i interface{}) {
	ApplicationContext().WireBean(i)
}

// Beans 获取所有 Bean 的定义，不能保证解析和注入，请谨慎使用该函数!
func Beans() []*core.BeanDefinition {
	return ApplicationContext().Beans()
}

func GetBean(i interface{}, selector ...bean.Selector) bool {
	return ApplicationContext().GetBean(i, selector...)
}

func FindBean(selector bean.Selector) (bean.Definition, bool) {
	return ApplicationContext().FindBean(selector)
}

func CollectBeans(i interface{}, selectors ...bean.Selector) bool {
	return ApplicationContext().CollectBeans(i, selectors...)
}

func Invoke(fn interface{}, args ...arg.Arg) error {
	return ApplicationContext().Invoke(fn, args...)
}

type GoFuncWithContext func(context.Context)

func Go(fn GoFuncWithContext) {
	appCtx := ApplicationContext()
	appCtx.Go(func() { fn(appCtx.Context()) })
}

func ObjBean(i interface{}) *core.BeanDefinition {
	bd := core.ObjBean(i)
	gApp.Bean(bd)
	return bd
}

func CtorBean(fn interface{}, args ...arg.Arg) *core.BeanDefinition {
	bd := core.CtorBean(fn, args...)
	gApp.Bean(bd)
	return bd
}

func Config(fn interface{}, args ...arg.Arg) *core.Configer {
	c := core.Config(fn, args...)
	gApp.Config(c)
	return c
}

// GRpcServer 注册 gRPC 服务提供者，fn 是 gRPC 自动生成的服务注册函数，serviceName 是服务名称，
// 必须对应 *_grpc.pg.go 文件里面 grpc.ServiceDesc 的 ServiceName 字段，server 是服务具体提供者对象。
func GRpcServer(fn interface{}, serviceName string, server interface{}) *grpc.Server {
	s := grpc.NewServer(fn, serviceName, server)
	gApp.GRpcServer(s)
	return s
}

// GRpcClient 注册 gRPC 服务客户端，fn 是 gRPC 自动生成的客户端构造函数
func GRpcClient(fn interface{}, endpoint string) *grpc.Client {
	c := grpc.NewClient(fn, endpoint)
	gApp.GRpcClient(c)
	return c
}

// Consume 注册 BIND 形式的消息消费者
func Consume(topic string, fn interface{}) {
	gApp.Consume(topic, fn)
}

// Route 返回和 app.WebMapper 绑定的路由分组
func Route(basePath string, filters ...bean.Selector) *app.WebRouter {
	return app.NewWebRouter(gApp.WebMapping, basePath, filters)
}

// HandleRequest 注册任意 HTTP 方法处理函数
func HandleRequest(method uint32, path string, fn web.Handler, filters ...bean.Selector) *app.WebMapper {
	return gApp.WebMapping.HandleRequest(method, path, fn, filters)
}

// MappingRequest 注册任意 HTTP 方法处理函数
func MappingRequest(method uint32, path string, fn web.HandlerFunc, filters ...bean.Selector) *app.WebMapper {
	return gApp.WebMapping.HandleRequest(method, path, web.FUNC(fn), filters)
}

// BindingRequest 注册任意 HTTP 方法处理函数
func BindingRequest(method uint32, path string, fn interface{}, filters ...bean.Selector) *app.WebMapper {
	return gApp.WebMapping.HandleRequest(method, path, web.BIND(fn), filters)
}

// HandleGet 注册 GET 方法处理函数
func HandleGet(path string, fn web.Handler, filters ...bean.Selector) *app.WebMapper {
	return HandleRequest(web.MethodGet, path, fn, filters...)
}

// MappingGet 注册 GET 方法处理函数
func MappingGet(path string, fn web.HandlerFunc, filters ...bean.Selector) *app.WebMapper {
	return HandleRequest(web.MethodGet, path, web.FUNC(fn), filters...)
}

// BindingGet 注册 GET 方法处理函数
func BindingGet(path string, fn interface{}, filters ...bean.Selector) *app.WebMapper {
	return HandleRequest(web.MethodGet, path, web.BIND(fn), filters...)
}

// HandlePost 注册 POST 方法处理函数
func HandlePost(path string, fn web.Handler, filters ...bean.Selector) *app.WebMapper {
	return HandleRequest(web.MethodPost, path, fn, filters...)
}

// MappingPost 注册 POST 方法处理函数
func MappingPost(path string, fn web.HandlerFunc, filters ...bean.Selector) *app.WebMapper {
	return HandleRequest(web.MethodPost, path, web.FUNC(fn), filters...)
}

// BindingPost 注册 POST 方法处理函数
func BindingPost(path string, fn interface{}, filters ...bean.Selector) *app.WebMapper {
	return HandleRequest(web.MethodPost, path, web.BIND(fn), filters...)
}

// HandlePut 注册 PUT 方法处理函数
func HandlePut(path string, fn web.Handler, filters ...bean.Selector) *app.WebMapper {
	return HandleRequest(web.MethodPut, path, fn, filters...)
}

// MappingPut 注册 PUT 方法处理函数
func MappingPut(path string, fn web.HandlerFunc, filters ...bean.Selector) *app.WebMapper {
	return HandleRequest(web.MethodPut, path, web.FUNC(fn), filters...)
}

// BindingPut 注册 PUT 方法处理函数
func BindingPut(path string, fn interface{}, filters ...bean.Selector) *app.WebMapper {
	return HandleRequest(web.MethodPut, path, web.BIND(fn), filters...)
}

// HandleDelete 注册 DELETE 方法处理函数
func HandleDelete(path string, fn web.Handler, filters ...bean.Selector) *app.WebMapper {
	return HandleRequest(web.MethodDelete, path, fn, filters...)
}

// MappingDelete 注册 DELETE 方法处理函数
func MappingDelete(path string, fn web.HandlerFunc, filters ...bean.Selector) *app.WebMapper {
	return HandleRequest(web.MethodDelete, path, web.FUNC(fn), filters...)
}

// BindingDelete 注册 DELETE 方法处理函数
func BindingDelete(path string, fn interface{}, filters ...bean.Selector) *app.WebMapper {
	return HandleRequest(web.MethodDelete, path, web.BIND(fn), filters...)
}
