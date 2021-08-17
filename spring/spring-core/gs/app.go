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
	"path/filepath"
	"reflect"
	"strings"
	"syscall"

	"github.com/go-spring/spring-boost/cast"
	"github.com/go-spring/spring-boost/log"
	"github.com/go-spring/spring-boost/util"
	"github.com/go-spring/spring-core/conf"
	"github.com/go-spring/spring-core/grpc"
	"github.com/go-spring/spring-core/gs/arg"
	"github.com/go-spring/spring-core/gs/environ"
	"github.com/go-spring/spring-core/mq"
	"github.com/go-spring/spring-core/web"
)

// AppRunner 导出 appRunner 类型
var AppRunner = (*appRunner)(nil)

// appRunner 命令行启动器接口
type appRunner interface {
	Run(ctx AppContext)
}

// AppEvent 导出 appEvent 类型
var AppEvent = (*appEvent)(nil)

// appEvent 应用运行过程中的事件
type appEvent interface {
	OnStopApp(ctx AppContext)  // 应用停止的事件
	OnStartApp(ctx AppContext) // 应用启动的事件
}

// WebRouter 导出 web.Router 类型
var WebRouter = (*web.Router)(nil)

// WebFilter 导出 web.Filter 类型
var WebFilter = (*web.Filter)(nil)

// App 应用
type App struct {

	// 应用上下文
	c *Container
	b *bootstrap

	banner    string
	router    web.Router
	consumers *Consumers

	exitChan chan struct{}

	Events  []appEvent  `autowire:""`
	Runners []appRunner `autowire:""`

	// 属性列表解析完成后的回调
	mapOfOnProperty map[string]interface{}
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
		c:               New(),
		b:               &bootstrap{c: New(), mapOfOnProperty: make(map[string]interface{})},
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

	<-app.exitChan

	app.c.Close()
	log.Info("application exited")
	return nil
}

func (app *App) start() (err error) {

	app.Object(app.consumers)
	app.Object(app.router).Export(WebRouter)
	app.Object(app)

	e := &environment{p: conf.New()}
	if err = e.prepare(); err != nil {
		return
	}

	showBanner := cast.ToBool(e.p.Get(environ.SpringBannerVisible))
	if showBanner {
		app.printBanner(app.getBanner(e.configLocations))
	}

	if err = app.b.start(e); err != nil {
		return
	}

	if err = app.profile(e); err != nil {
		return
	}

	sourceMap, err := app.b.sourceMap(e)
	if err != nil {
		return
	}

	// 保存远程 default 配置。
	for _, p := range sourceMap[""] {
		for _, k := range p.Keys() {
			app.c.p.Set(k, p.Get(k))
		}
	}

	// 保存远程 active 配置。
	for _, p := range sourceMap[e.activeProfile] {
		for _, k := range p.Keys() {
			app.c.p.Set(k, p.Get(k))
		}
	}

	// 保存从环境变量和命令行解析的属性
	for _, k := range e.p.Keys() {
		app.c.p.Set(k, e.p.Get(k))
	}

	for key, f := range app.mapOfOnProperty {
		t := reflect.TypeOf(f)
		in := reflect.New(t.In(0)).Elem()
		err = app.c.p.Bind(in, conf.Key(key))
		if err != nil {
			return
		}
		reflect.ValueOf(f).Call([]reflect.Value{in})
	}

	if err = app.c.refresh(); err != nil {
		return
	}

	ctx := &pandora{app.c}

	// 执行命令行启动器
	for _, r := range app.Runners {
		r.Run(ctx)
	}

	// 通知应用启动事件
	for _, e := range app.Events {
		e.OnStartApp(ctx)
	}

	// 通知应用停止事件
	app.c.safeGo(func(c context.Context) {
		select {
		case <-c.Done():
			for _, e := range app.Events {
				e.OnStopApp(ctx)
			}
		}
	})

	if !app.c.enablePandora() {
		app.c.clearCache()
	}

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
	if n := (maxLength - len(environ.Version)) / 2; n > 0 {
		padding = make([]byte, n)
		for i := range padding {
			padding[i] = ' '
		}
	}
	fmt.Println(string(padding) + environ.Version + "\n")
}

func (app *App) profile(e *environment) error {
	if err := app.loadConfigFile(e, "application"); err != nil {
		return err
	}
	if e.activeProfile == "" {
		return nil
	}
	return app.loadConfigFile(e, "application-"+e.activeProfile)
}

func (app *App) loadConfigFile(e *environment, filename string) error {
	for _, loc := range e.configLocations {
		for _, ext := range e.configExtensions {
			err := app.c.p.Load(filepath.Join(loc, filename+ext))
			if err != nil && !os.IsNotExist(err) {
				return err
			}
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

// OnProperty 当 key 对应的属性值准备好后发送一个通知。
func (app *App) OnProperty(key string, fn interface{}) {
	err := validOnProperty(fn)
	util.Panic(err).When(err != nil)
	app.mapOfOnProperty[key] = fn
}

// Property 设置 key 对应的属性值，如果 key 对应的属性值已经存在则 Set 方法会
// 覆盖旧值。Set 方法除了支持 string 类型的属性值，还支持 int、uint、bool 等
// 其他基础数据类型的属性值。特殊情况下，Set 方法也支持 slice 、map 与基础数据
// 类型组合构成的属性值，其处理方式是将组合结构层层展开，可以将组合结构看成一棵树，
// 那么叶子结点的路径就是属性的 key，叶子结点的值就是属性的值。
func (app *App) Property(key string, value interface{}) {
	app.c.Property(key, value)
}

// Object 注册对象形式的 bean ，需要注意的是该方法在注入开始后就不能再调用了。
func (app *App) Object(i interface{}) *BeanDefinition {
	return app.c.register(NewBean(reflect.ValueOf(i)))
}

// Provide 注册构造函数形式的 bean ，需要注意的是该方法在注入开始后就不能再调用了。
func (app *App) Provide(ctor interface{}, args ...arg.Arg) *BeanDefinition {
	return app.c.register(NewBean(ctor, args...))
}

// HandleGet 注册 GET 方法处理函数
func (app *App) HandleGet(path string, h web.Handler) *web.Mapper {
	return app.router.HandleGet(path, h)
}

// GetMapping 注册 GET 方法处理函数
func (app *App) GetMapping(path string, fn web.HandlerFunc) *web.Mapper {
	return app.router.GetMapping(path, fn)
}

// GetBinding 注册 GET 方法处理函数
func (app *App) GetBinding(path string, fn interface{}) *web.Mapper {
	return app.router.GetBinding(path, fn)
}

// HandlePost 注册 POST 方法处理函数
func (app *App) HandlePost(path string, h web.Handler) *web.Mapper {
	return app.router.HandlePost(path, h)
}

// PostMapping 注册 POST 方法处理函数
func (app *App) PostMapping(path string, fn web.HandlerFunc) *web.Mapper {
	return app.router.PostMapping(path, fn)
}

// PostBinding 注册 POST 方法处理函数
func (app *App) PostBinding(path string, fn interface{}) *web.Mapper {
	return app.router.PostBinding(path, fn)
}

// HandlePut 注册 PUT 方法处理函数
func (app *App) HandlePut(path string, h web.Handler) *web.Mapper {
	return app.router.HandlePut(path, h)
}

// PutMapping 注册 PUT 方法处理函数
func (app *App) PutMapping(path string, fn web.HandlerFunc) *web.Mapper {
	return app.router.PutMapping(path, fn)
}

// PutBinding 注册 PUT 方法处理函数
func (app *App) PutBinding(path string, fn interface{}) *web.Mapper {
	return app.router.PutBinding(path, fn)
}

// HandleDelete 注册 DELETE 方法处理函数
func (app *App) HandleDelete(path string, h web.Handler) *web.Mapper {
	return app.router.HandleDelete(path, h)
}

// DeleteMapping 注册 DELETE 方法处理函数
func (app *App) DeleteMapping(path string, fn web.HandlerFunc) *web.Mapper {
	return app.router.DeleteMapping(path, fn)
}

// DeleteBinding 注册 DELETE 方法处理函数
func (app *App) DeleteBinding(path string, fn interface{}) *web.Mapper {
	return app.router.DeleteBinding(path, web.BIND(fn))
}

// HandleRequest 注册任意 HTTP 方法处理函数
func (app *App) HandleRequest(method uint32, path string, h web.Handler) *web.Mapper {
	return app.router.HandleRequest(method, path, h)
}

// RequestMapping 注册任意 HTTP 方法处理函数
func (app *App) RequestMapping(method uint32, path string, fn web.HandlerFunc) *web.Mapper {
	return app.router.RequestMapping(method, path, fn)
}

// RequestBinding 注册任意 HTTP 方法处理函数
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
