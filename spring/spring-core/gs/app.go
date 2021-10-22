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

package gs

import (
	"context"
	"fmt"
	"io/ioutil"
	"os"
	"os/signal"
	"path/filepath"
	"reflect"
	"strings"
	"syscall"

	"github.com/go-spring/spring-base/cast"
	"github.com/go-spring/spring-base/conf"
	"github.com/go-spring/spring-base/log"
	"github.com/go-spring/spring-base/util"
	"github.com/go-spring/spring-core/grpc"
	"github.com/go-spring/spring-core/gs/arg"
	"github.com/go-spring/spring-core/gs/internal"
	"github.com/go-spring/spring-core/mq"
	"github.com/go-spring/spring-core/web"
)

// SpringBannerVisible 是否显示 banner。
const SpringBannerVisible = "spring.banner.visible"

type AppContext Environment

// AppRunner 命令行启动器接口
type AppRunner interface {
	Run(ctx AppContext)
}

// AppEvent 应用运行过程中的事件
type AppEvent interface {
	OnAppStart(ctx AppContext)     // 应用启动的事件
	OnAppStop(ctx context.Context) // 应用停止的事件
}

type tempApp struct {
	banner          string
	router          web.Router
	consumers       *Consumers
	grpcServers     *GrpcServers
	mapOfOnProperty map[string]interface{} // 属性列表解析完成后的回调
	Runners         []AppRunner            `autowire:"${command-line-runner.collection:=*?}"`
}

// App 应用
type App struct {
	*tempApp

	c *container
	b *bootstrap

	exitChan chan struct{}

	Events []AppEvent `autowire:"${application-event.collection:=*?}"`
}

type Consumers struct {
	consumers []mq.Consumer
}

func (c *Consumers) Add(consumer mq.Consumer) {
	c.consumers = append(c.consumers, consumer)
}

func (c *Consumers) ForEach(fn func(mq.Consumer)) {
	for _, consumer := range c.consumers {
		fn(consumer)
	}
}

type GrpcServers struct {
	servers map[string]*grpc.Server
}

func (s *GrpcServers) Add(serviceName string, server *grpc.Server) {
	s.servers[serviceName] = server
}

func (s *GrpcServers) ForEach(fn func(string, *grpc.Server)) {
	for serviceName, server := range s.servers {
		fn(serviceName, server)
	}
}

// NewApp application 的构造函数
func NewApp() *App {
	return &App{
		c: New().(*container),
		tempApp: &tempApp{
			router:    web.NewRouter(),
			consumers: new(Consumers),
			grpcServers: &GrpcServers{
				servers: map[string]*grpc.Server{},
			},
			mapOfOnProperty: make(map[string]interface{}),
		},
		exitChan: make(chan struct{}),
	}
}

// Banner 自定义 banner 字符串。
func (app *App) Banner(banner string) {
	app.banner = banner
}

func (app *App) Run() error {

	// 响应控制台的 Ctrl+C 及 kill 命令。
	go func() {
		ch := make(chan os.Signal, 1)
		signal.Notify(ch, os.Interrupt, syscall.SIGTERM)
		sig := <-ch
		app.ShutDown(fmt.Errorf("signal %v", sig))
	}()

	if err := app.start(); err != nil {
		return err
	}

	<-app.exitChan

	if app.b != nil {
		app.b.c.Close()
	}

	app.c.Close()
	log.Info("application exited")
	return nil
}

func (app *App) clear() {
	app.c.clear()
	if app.b != nil {
		app.b.clear()
	}
	app.tempApp = nil
}

func (app *App) start() error {

	app.Object(app)
	app.Object(app.consumers)
	app.Object(app.grpcServers)
	app.Object(app.router).Export((*web.Router)(nil))

	e := &configuration{
		p:               conf.New(),
		resourceLocator: new(defaultResourceLocator),
	}

	if err := e.prepare(); err != nil {
		return err
	}

	showBanner := cast.ToBool(e.p.Get(SpringBannerVisible))
	if showBanner {
		app.printBanner(app.getBanner(e))
	}

	if app.b != nil {
		if err := app.b.start(e); err != nil {
			return err
		}
	}

	if err := app.loadProperties(e); err != nil {
		return err
	}

	// 保存从环境变量和命令行解析的属性
	for _, k := range e.p.Keys() {
		app.c.p.Set(k, e.p.Get(k))
	}

	for key, f := range app.mapOfOnProperty {
		t := reflect.TypeOf(f)
		in := reflect.New(t.In(0)).Elem()
		err := app.c.p.Bind(in, conf.Key(key))
		if err != nil {
			return err
		}
		reflect.ValueOf(f).Call([]reflect.Value{in})
	}

	if err := app.c.Refresh(internal.AutoClear(false)); err != nil {
		return err
	}

	// 执行命令行启动器
	for _, r := range app.Runners {
		r.Run(app.c)
	}

	// 通知应用启动事件
	for _, event := range app.Events {
		event.OnAppStart(app.c)
	}

	app.clear()

	// 通知应用停止事件
	app.c.Go(func(c context.Context) {
		<-c.Done()
		ctx := context.TODO()
		for _, event := range app.Events {
			event.OnAppStop(ctx)
		}
	})

	log.Info("application started successfully")
	return nil
}

const DefaultBanner = `
                                              (_)              
  __ _    ___             ___   _ __    _ __   _   _ __     __ _ 
 / _' |  / _ \   ______  / __| | '_ \  | '__| | | | '_ \   / _' |
| (_| | | (_) | |______| \__ \ | |_) | | |    | | | | | | | (_| |
 \__, |  \___/           |___/ | .__/  |_|    |_| |_| |_|  \__, |
  __/ |                        | |                          __/ |
 |___/                         |_|                         |___/ 
`

func (app *App) getBanner(e *configuration) string {
	if app.banner != "" {
		return app.banner
	}
	resources, err := e.resourceLocator.Locate("banner.txt")
	if err != nil {
		return ""
	}
	banner := DefaultBanner
	for _, resource := range resources {
		if b, _ := ioutil.ReadAll(resource); b != nil {
			banner = string(b)
		}
	}
	return banner
}

// printBanner 打印 banner 到控制台
func (app *App) printBanner(banner string) {

	if banner[0] != '\n' {
		fmt.Println()
	}

	maxLength := 0
	for _, s := range strings.Split(banner, "\n") {
		fmt.Printf("\x1b[36m%s\x1b[0m\n", s) // CYAN
		if len(s) > maxLength {
			maxLength = len(s)
		}
	}

	if banner[len(banner)-1] != '\n' {
		fmt.Println()
	}

	var padding []byte
	if n := (maxLength - len(Version)) / 2; n > 0 {
		padding = make([]byte, n)
		for i := range padding {
			padding[i] = ' '
		}
	}
	fmt.Println(string(padding) + Version + "\n")
}

func (app *App) loadProperties(e *configuration) error {
	var resources []Resource

	for _, ext := range e.ConfigExtensions {
		sources, err := app.loadResource(e, "application"+ext)
		if err != nil {
			return err
		}
		resources = append(resources, sources...)
	}

	for _, profile := range e.ActiveProfiles {
		for _, ext := range e.ConfigExtensions {
			sources, err := app.loadResource(e, "application-"+profile+ext)
			if err != nil {
				return err
			}
			resources = append(resources, sources...)
		}
	}

	for _, resource := range resources {
		b, err := ioutil.ReadAll(resource)
		if err != nil {
			return err
		}
		p, err := conf.Bytes(b, filepath.Ext(resource.Name()))
		if err != nil {
			return err
		}
		for _, key := range p.Keys() {
			app.c.p.Set(key, p.Get(key))
		}
	}

	return nil
}

func (app *App) loadResource(e *configuration, filename string) ([]Resource, error) {

	var locators []ResourceLocator
	locators = append(locators, e.resourceLocator)
	if app.b != nil {
		locators = append(locators, app.b.resourceLocators...)
	}

	var resources []Resource
	for _, locator := range locators {
		sources, err := locator.Locate(filename)
		if err != nil {
			return nil, err
		}
		resources = append(resources, sources...)
	}
	return resources, nil
}

// ShutDown 关闭执行器
func (app *App) ShutDown(err error) {
	log.Infof("program will exit %s", err.Error())
	select {
	case <-app.exitChan:
		// chan 已关闭，无需再次关闭。
	default:
		close(app.exitChan)
	}
}

// Bootstrap 返回 *bootstrap 对象。
func (app *App) Bootstrap() *bootstrap {
	if app.b == nil {
		app.b = newBootstrap()
	}
	return app.b
}

// OnProperty 当 key 对应的属性值准备好后发送一个通知。
func (app *App) OnProperty(key string, fn interface{}) {
	err := validOnProperty(fn)
	util.Panic(err).When(err != nil)
	app.mapOfOnProperty[key] = fn
}

// Property 参考 Container.Property 的解释。
func (app *App) Property(key string, value interface{}) {
	app.c.Property(key, value)
}

// Object 参考 Container.Object 的解释。
func (app *App) Object(i interface{}) *BeanDefinition {
	return app.c.register(NewBean(reflect.ValueOf(i)))
}

// Provide 参考 Container.Provide 的解释。
func (app *App) Provide(ctor interface{}, args ...arg.Arg) *BeanDefinition {
	return app.c.register(NewBean(ctor, args...))
}

// HandleGet 注册 GET 方法处理函数。
func (app *App) HandleGet(path string, h web.Handler) *web.Mapper {
	return app.router.HandleGet(path, h)
}

// GetMapping 注册 GET 方法处理函数。
func (app *App) GetMapping(path string, fn web.HandlerFunc) *web.Mapper {
	return app.router.GetMapping(path, fn)
}

// GetBinding 注册 GET 方法处理函数。
func (app *App) GetBinding(path string, fn interface{}) *web.Mapper {
	return app.router.GetBinding(path, fn)
}

// HandlePost 注册 POST 方法处理函数。
func (app *App) HandlePost(path string, h web.Handler) *web.Mapper {
	return app.router.HandlePost(path, h)
}

// PostMapping 注册 POST 方法处理函数。
func (app *App) PostMapping(path string, fn web.HandlerFunc) *web.Mapper {
	return app.router.PostMapping(path, fn)
}

// PostBinding 注册 POST 方法处理函数。
func (app *App) PostBinding(path string, fn interface{}) *web.Mapper {
	return app.router.PostBinding(path, fn)
}

// HandlePut 注册 PUT 方法处理函数。
func (app *App) HandlePut(path string, h web.Handler) *web.Mapper {
	return app.router.HandlePut(path, h)
}

// PutMapping 注册 PUT 方法处理函数。
func (app *App) PutMapping(path string, fn web.HandlerFunc) *web.Mapper {
	return app.router.PutMapping(path, fn)
}

// PutBinding 注册 PUT 方法处理函数。
func (app *App) PutBinding(path string, fn interface{}) *web.Mapper {
	return app.router.PutBinding(path, fn)
}

// HandleDelete 注册 DELETE 方法处理函数。
func (app *App) HandleDelete(path string, h web.Handler) *web.Mapper {
	return app.router.HandleDelete(path, h)
}

// DeleteMapping 注册 DELETE 方法处理函数。
func (app *App) DeleteMapping(path string, fn web.HandlerFunc) *web.Mapper {
	return app.router.DeleteMapping(path, fn)
}

// DeleteBinding 注册 DELETE 方法处理函数。
func (app *App) DeleteBinding(path string, fn interface{}) *web.Mapper {
	return app.router.DeleteBinding(path, web.BIND(fn))
}

// HandleRequest 注册任意 HTTP 方法处理函数。
func (app *App) HandleRequest(method uint32, path string, h web.Handler) *web.Mapper {
	return app.router.HandleRequest(method, path, h)
}

// RequestMapping 注册任意 HTTP 方法处理函数。
func (app *App) RequestMapping(method uint32, path string, fn web.HandlerFunc) *web.Mapper {
	return app.router.RequestMapping(method, path, fn)
}

// RequestBinding 注册任意 HTTP 方法处理函数。
func (app *App) RequestBinding(method uint32, path string, fn interface{}) *web.Mapper {
	return app.router.RequestBinding(method, path, fn)
}

// Consume 注册 MQ 消费者。
func (app *App) Consume(fn interface{}, topics ...string) {
	app.consumers.Add(mq.Bind(fn, topics...))
}

// GrpcServer 注册 gRPC 服务提供者，fn 是 gRPC 自动生成的服务注册函数，
// serviceName 是服务名称，必须对应 *_grpc.pg.go 文件里面 grpc.ServerDesc
// 的 ServiceName 字段，server 是服务提供者对象。
func (app *App) GrpcServer(serviceName string, server *grpc.Server) {
	app.grpcServers.Add(serviceName, server)
}

// GrpcClient 注册 gRPC 服务客户端，fn 是 gRPC 自动生成的客户端构造函数。
func (app *App) GrpcClient(fn interface{}, endpoint string) *BeanDefinition {
	return app.c.register(NewBean(fn, endpoint))
}
