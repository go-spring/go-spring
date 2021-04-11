package boot

import (
	"context"

	"github.com/go-spring/spring-core/arg"
	"github.com/go-spring/spring-core/bean"
	"github.com/go-spring/spring-core/gs"
	"github.com/go-spring/spring-core/mq"
	"github.com/go-spring/spring-core/web"
)

var gApp = NewApplication()

func ExpectSysProperties(pattern ...string) {
	gApp.expectSysProperties = pattern
}

func BannerMode(mode int) {
	gApp.bannerMode = mode
}

// Banner 设置自定义 Banner 字符串
func Banner(banner string) {
	gApp.banner = banner
}

// GetProfile 返回运行环境
func GetProfile() string {
	return gApp.appCtx.GetProfile()
}

// Profile 设置运行环境
func Profile(profile string) {
	gApp.appCtx.Profile(profile)
}

func GetProperty(key string) interface{} {
	return gApp.appCtx.GetProperty(key)
}

// Property 设置属性值，属性名称统一转成小写。
func Property(key string, value interface{}) {
	gApp.appCtx.Property(key, value)
}

func Config(fn interface{}, args ...arg.Arg) *gs.Configer {
	return gApp.appCtx.Config(fn, args...)
}

// Object 注册对象形式的 Bean。
func Object(i interface{}) *gs.BeanDefinition {
	return gApp.appCtx.Object(i)
}

// Factory 注册构造函数形式的 Bean。
func Factory(fn interface{}, args ...arg.Arg) *gs.BeanDefinition {
	return gApp.appCtx.Factory(fn, args...)
}

// WireBean 对对象或者构造函数的结果进行依赖注入和属性绑定，返回处理后的对象
func WireBean(objOrCtor interface{}, ctorArgs ...arg.Arg) (interface{}, error) {
	return gApp.appCtx.WireBean(objOrCtor, ctorArgs...)
}

func GetBean(i interface{}, selector ...bean.Selector) error {
	return gApp.appCtx.GetBean(i, selector...)
}

// FindBean 返回符合条件的 Bean 集合，不保证返回的 Bean 已经完成注入和绑定过程。
func FindBean(selector bean.Selector) ([]bean.Definition, error) {
	return gApp.appCtx.FindBean(selector)
}

func CollectBeans(i interface{}, selectors ...bean.Selector) error {
	return gApp.appCtx.CollectBeans(i, selectors...)
}

func Run(fn interface{}, args ...arg.Arg) error {
	return gApp.appCtx.Run(fn, args...)
}

type GoFuncWithContext func(context.Context)

func Go(fn GoFuncWithContext) {
	appCtx := gApp.appCtx
	appCtx.Go(func() { fn(appCtx.Context()) })
}

///////////////////////////////////////// GRPC ////////////////////////////////////////

type GRpcService struct {
	ServiceName string      // 服务的名称
	Handler     interface{} // 服务注册函数
	Server      interface{} // 服务提供者
}

var GRpcServers map[interface{}]*GRpcService // gRPC 服务列表

// GRpcServer 注册 gRPC 服务提供者，fn 是 gRPC 自动生成的服务注册函数，serviceName 是服务名称，
// 必须对应 *_grpc.pg.go 文件里面 grpc.ServiceDesc 的 ServiceName 字段，server 是服务具体提供者对象。
func GRpcServer(serviceName string, fn interface{}, server interface{}) {
	s := &GRpcService{Handler: fn, Server: server, ServiceName: serviceName}
	GRpcServers[s.Handler] = s
}

// GRpcClient 注册 gRPC 服务客户端，fn 是 gRPC 自动生成的客户端构造函数
func GRpcClient(fn interface{}, endpoint string) *gs.BeanDefinition {
	return gApp.appCtx.Factory(fn, endpoint)
}

///////////////////////////////////////// Web /////////////////////////////////////////

var RootRouter = web.NewRootRouter()

// Route 返回和 RootRouter 绑定的路由分组
func Route(basePath string) *web.Router {
	return RootRouter.Route(basePath)
}

// HandleRequest 注册任意 HTTP 方法处理函数
func HandleRequest(method uint32, path string, fn web.Handler) *web.Mapper {
	return RootRouter.HandleRequest(method, path, fn)
}

// RequestMapping 注册任意 HTTP 方法处理函数
func RequestMapping(method uint32, path string, fn web.HandlerFunc) *web.Mapper {
	return RootRouter.HandleRequest(method, path, web.FUNC(fn))
}

// RequestBinding 注册任意 HTTP 方法处理函数
func RequestBinding(method uint32, path string, fn interface{}) *web.Mapper {
	return RootRouter.HandleRequest(method, path, web.BIND(fn))
}

// HandleGet 注册 GET 方法处理函数
func HandleGet(path string, fn web.Handler) *web.Mapper {
	return RootRouter.HandleGet(path, fn)
}

// GetMapping 注册 GET 方法处理函数
func GetMapping(path string, fn web.HandlerFunc) *web.Mapper {
	return RootRouter.HandleGet(path, web.FUNC(fn))
}

// GetBinding 注册 GET 方法处理函数
func GetBinding(path string, fn interface{}) *web.Mapper {
	return RootRouter.HandleGet(path, web.BIND(fn))
}

// HandlePost 注册 POST 方法处理函数
func HandlePost(path string, fn web.Handler) *web.Mapper {
	return RootRouter.HandlePost(path, fn)
}

// PostMapping 注册 POST 方法处理函数
func PostMapping(path string, fn web.HandlerFunc) *web.Mapper {
	return RootRouter.HandlePost(path, web.FUNC(fn))
}

// PostBinding 注册 POST 方法处理函数
func PostBinding(path string, fn interface{}) *web.Mapper {
	return RootRouter.HandlePost(path, web.BIND(fn))
}

// HandlePut 注册 PUT 方法处理函数
func HandlePut(path string, fn web.Handler) *web.Mapper {
	return RootRouter.HandlePut(path, fn)
}

// PutMapping 注册 PUT 方法处理函数
func PutMapping(path string, fn web.HandlerFunc) *web.Mapper {
	return RootRouter.HandlePut(path, web.FUNC(fn))
}

// PutBinding 注册 PUT 方法处理函数
func PutBinding(path string, fn interface{}) *web.Mapper {
	return RootRouter.HandlePut(path, web.BIND(fn))
}

// HandleDelete 注册 DELETE 方法处理函数
func HandleDelete(path string, fn web.Handler) *web.Mapper {
	return RootRouter.HandleDelete(path, fn)
}

// DeleteMapping 注册 DELETE 方法处理函数
func DeleteMapping(path string, fn web.HandlerFunc) *web.Mapper {
	return RootRouter.HandleDelete(path, web.FUNC(fn))
}

// DeleteBinding 注册 DELETE 方法处理函数
func DeleteBinding(path string, fn interface{}) *web.Mapper {
	return RootRouter.HandleDelete(path, web.BIND(fn))
}

// NewFilter 注册 web.Filter 对象
func NewFilter(objOrCtor interface{}, ctorArgs ...arg.Arg) *gs.BeanDefinition {
	return gApp.appCtx.NewBean(objOrCtor, ctorArgs...).Export((*web.Filter)(nil))
}

///////////////////////////////////////// MQ //////////////////////////////////////////

// Consumers MQ 消费者列表
var Consumers = make(map[string]*mq.BindConsumer)

// Consume 注册 BIND 形式的消息消费者
func Consume(topic string, fn interface{}) {
	Consumers[topic] = mq.BIND(topic, fn)
}
