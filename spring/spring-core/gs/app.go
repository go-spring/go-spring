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
	"path"
	"reflect"
	"strings"
	"syscall"

	"github.com/go-spring/spring-boost/cast"
	"github.com/go-spring/spring-boost/log"
	"github.com/go-spring/spring-boost/util"
	"github.com/go-spring/spring-core/conf"
	"github.com/go-spring/spring-core/grpc"
	"github.com/go-spring/spring-core/gs/arg"
	"github.com/go-spring/spring-core/mq"
	"github.com/go-spring/spring-core/web"
)

const (
	Version = "go-spring@v1.0.5"
	Website = "https://go-spring.com/"
)

// SpringBannerVisible 是否显示 banner。
const SpringBannerVisible = "spring.banner.visible"

// AppRunner 命令行启动器接口
type AppRunner interface {
	Run(ctx Environment)
}

// AppEvent 应用运行过程中的事件
type AppEvent interface {
	OnStopApp(ctx Environment)  // 应用停止的事件
	OnStartApp(ctx Environment) // 应用启动的事件
}

// App 应用
type App struct {
	Name string `value:"${spring.application.name}"`

	c *Container
	b *bootstrap

	banner    string
	router    web.Router
	consumers *Consumers

	exitChan chan struct{}

	Events  []AppEvent  `autowire:""`
	Runners []AppRunner `autowire:""`

	mapOfOnProperty map[string]interface{} // 属性列表解析完成后的回调
	propertySources []*propertySource
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

// NewApp application 的构造函数
func NewApp() *App {
	return &App{
		b: &bootstrap{
			c:                New(),
			mapOfOnProperty:  make(map[string]interface{}),
			resourceLocators: []ResourceLocator{},
		},
		c:               New(),
		mapOfOnProperty: make(map[string]interface{}),
		exitChan:        make(chan struct{}),
		router:          web.NewRouter(),
		consumers:       new(Consumers),
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

	app.c.ClearCache()

	<-app.exitChan

	if app.b != nil {
		app.b.c.Close()
	}

	app.c.Close()
	log.Info("application exited")
	return nil
}

func (app *App) start() error {

	app.b.Object(new(defaultResourceLocator)).
		Export((*ResourceLocator)(nil)).
		Order(HighestOrder)

	app.Object(app)
	app.Object(app.router)
	app.Object(app.consumers)

	e := &configuration{p: conf.New()}
	if err := e.prepare(); err != nil {
		return err
	}

	for _, ps := range app.propertySources {
		files, err := app.b.LoadResources(ps.file)
		if err != nil {
			return err
		}
		p := conf.New()
		for _, file := range files {
			if err = p.Load(file.Name()); err != nil {
				return err
			}
		}
		err = p.Bind(ps.object, conf.Key(ps.prefix))
		if err != nil {
			return err
		}
		app.Object(ps.object)
	}

	showBanner := cast.ToBool(e.p.Get(SpringBannerVisible))
	if showBanner {
		app.printBanner(app.getBanner(e.configLocations))
	}

	if err := app.b.start(e); err != nil {
		return err
	}

	if err := app.profile(e); err != nil {
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

	if err := app.c.Refresh(); err != nil {
		return err
	}

	// 执行命令行启动器
	for _, r := range app.Runners {
		r.Run(app.c.e)
	}

	// 通知应用启动事件
	for _, e := range app.Events {
		e.OnStartApp(app.c.e)
	}

	// 通知应用停止事件
	app.c.safeGo(func(c context.Context) {
		select {
		case <-c.Done():
			for _, e := range app.Events {
				e.OnStopApp(app.c.e)
			}
		}
	})

	log.Info("application started successfully")
	return nil
}

const DefaultBanner = `
 _______  _______         _______  _______  _______ _________ _        _______ 
(  ____ \(  ___  )       (  ____ \(  ____ )(  ____ )\__   __/( (    /|(  ____ \
| (    \/| (   ) |       | (    \/| (    )|| (    )|   ) (   |  \  ( || (    \/
| |      | |   | | _____ | (_____ | (____)|| (____)|   | |   |   \ | || |      
| | ____ | |   | |(_____)(_____  )|  _____)|     __)   | |   | (\ \) || | ____ 
| | \_  )| |   | |             ) || (      | (\ (      | |   | | \   || | \_  )
| (___) || (___) |       /\____) || )      | ) \ \_____) (___| )  \  || (___) |
(_______)(_______)       \_______)|/       |/   \__/\_______/|/    )_)(_______)
`

func (app *App) getBanner(configLocations []string) string {
	if app.banner != "" {
		return app.banner
	}
	for _, configLocation := range configLocations {
		file := path.Join(configLocation, "banner.txt")
		if b, err := ioutil.ReadFile(file); err == nil {
			return string(b)
		}
	}
	return DefaultBanner
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

func (app *App) profile(e *configuration) error {
	var files []*os.File

	for _, locator := range app.b.resourceLocators {
		for _, ext := range e.configExtensions {
			sources, err := locator.Locate("application" + ext)
			if err != nil {
				return err
			}
			files = append(files, sources...)
		}
	}

	for _, profile := range e.activeProfiles {
		for _, locator := range app.b.resourceLocators {
			for _, ext := range e.configExtensions {
				sources, err := locator.Locate("application-" + profile + ext)
				if err != nil {
					return err
				}
				files = append(files, sources...)
			}
		}
	}

	for _, file := range files {
		p, err := conf.Load(file.Name())
		if err != nil {
			return err
		}
		for _, key := range p.Keys() {
			app.c.p.Set(key, p.Get(key))
		}
	}

	return nil
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
	return app.b
}

func (app *App) PropertySource(file string, prefix string, object interface{}) {
	ps := &propertySource{file: file, prefix: prefix, object: object}
	app.propertySources = append(app.propertySources, ps)
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

// GrpcClient 注册 gRPC 服务客户端，fn 是 gRPC 自动生成的客户端构造函数。
func (app *App) GrpcClient(fn interface{}, endpoint string) *BeanDefinition {
	return app.c.register(NewBean(fn, endpoint))
}

// GrpcServer 注册 gRPC 服务提供者，fn 是 gRPC 自动生成的服务注册函数，
// serviceName 是服务名称，必须对应 *_grpc.pg.go 文件里面 grpc.ServerDesc
// 的 ServiceName 字段，server 是服务提供者对象。
func (app *App) GrpcServer(serviceName string, fn interface{}, service interface{}) *BeanDefinition {
	s := &grpc.Server{Service: service, Register: fn}
	return app.c.register(NewBean(s)).Name(serviceName)
}
