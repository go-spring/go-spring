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
	"os"
	"os/signal"
	"reflect"
	"strings"
	"syscall"

	"github.com/go-spring/spring-core/grpc"
	"github.com/go-spring/spring-core/gs/arg"
	"github.com/go-spring/spring-core/mq"
	"github.com/go-spring/spring-core/web"
)

// SpringBannerVisible 是否显示 banner。
const SpringBannerVisible = "spring.banner.visible"

// AppRunner 命令行启动器接口
type AppRunner interface {
	Run(ctx Context)
}

// AppEvent 应用运行过程中的事件
type AppEvent interface {
	OnAppStart(ctx Context)        // 应用启动的事件
	OnAppStop(ctx context.Context) // 应用停止的事件
}

type tempApp struct {
	router      web.Router
	consumers   *Consumers
	grpcServers *GrpcServers
	banner      string
}

// App 应用
type App struct {
	*tempApp

	c *container
	b *Bootstrapper

	exitChan chan struct{}

	Events  []AppEvent  `autowire:"${application-event.collection:=*?}"`
	Runners []AppRunner `autowire:"${command-line-runner.collection:=*?}"`
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
		},
		exitChan: make(chan struct{}),
	}
}

// Banner 自定义 banner 字符串。
func (app *App) Banner(banner string) {
	app.banner = banner
}

func (app *App) Start() (Context, error) {

	app.Object(app)
	app.Object(app.consumers)
	app.Object(app.grpcServers)
	app.Object(app.router).Export((*web.Router)(nil))

	if err := app.start(); err != nil {
		return nil, err
	}
	return app.c, nil
}

func (app *App) Stop() {

	// if app.b != nil {
	// 	app.b.c.Close()
	// }

	app.c.Close()
}

func (app *App) Run() error {
	_, err := app.Start()
	if err != nil {
		return err
	}

	go func() {
		ch := make(chan os.Signal, 1)
		signal.Notify(ch, os.Interrupt, syscall.SIGTERM)
		sig := <-ch
		app.ShutDown(fmt.Sprintf("signal %v", sig))
	}()

	<-app.exitChan
	return nil
}

func (app *App) start() error {

	// showBanner, _ := strconv.ParseBool(e.p.Get(SpringBannerVisible))
	// if showBanner {
	// 	app.printBanner(app.getBanner(e))
	// }

	// if app.b != nil {
	// 	if err := app.b.start(e); err != nil {
	// 		return err
	// 	}
	// }
	//
	// if err := app.loadProperties(e); err != nil {
	// 	return err
	// }

	// // 保存从环境变量和命令行解析的属性
	// for _, k := range e.p.Keys() {
	// 	app.c.initProperties.Set(k, e.p.Get(k))
	// }

	if err := app.c.refresh(false); err != nil {
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

	// 通知应用停止事件
	app.c.Go(func(ctx context.Context) {
		<-ctx.Done()
		for _, event := range app.Events {
			event.OnAppStop(context.Background())
		}
	})

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

func (app *App) getBanner() string {
	if app.banner != "" {
		return app.banner
	}
	banner := DefaultBanner
	// for _, resource := range resources {
	// 	if b, _ := ioutil.ReadAll(resource); b != nil {
	// 		banner = string(b)
	// 	}
	// }
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

// ShutDown 关闭执行器
func (app *App) ShutDown(msg ...string) {
	select {
	case <-app.exitChan:
		// chan 已关闭，无需再次关闭。
	default:
		close(app.exitChan)
	}
}

// Bootstrap 返回 *bootstrap 对象。
func (app *App) Bootstrap() *Bootstrapper {
	if app.b == nil {
		app.b = newBootstrap()
	}
	return app.b
}

// OnProperty 当 key 对应的属性值准备好后发送一个通知。
func (app *App) OnProperty(key string, fn interface{}) {
	app.c.OnProperty(key, fn)
}

// Property 参考 Container.Property 的解释。
func (app *App) Property(key string, value interface{}) {
	app.c.Property(key, value)
}

// Accept 参考 Container.Accept 的解释。
func (app *App) Accept(b *BeanDefinition) *BeanDefinition {
	return app.c.Accept(b)
}

// Object 参考 Container.Object 的解释。
func (app *App) Object(i interface{}) *BeanDefinition {
	return app.c.Accept(NewBean(reflect.ValueOf(i)))
}

// Provide 参考 Container.Provide 的解释。
func (app *App) Provide(ctor interface{}, args ...arg.Arg) *BeanDefinition {
	return app.c.Accept(NewBean(ctor, args...))
}
