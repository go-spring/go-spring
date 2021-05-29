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
	"regexp"
	"strings"
	"syscall"

	"github.com/go-spring/spring-core/arg"
	"github.com/go-spring/spring-core/conf"
	"github.com/go-spring/spring-core/conf/k8s"
	"github.com/go-spring/spring-core/grpc"
	"github.com/go-spring/spring-core/log"
	"github.com/go-spring/spring-core/mq"
	"github.com/go-spring/spring-core/util"
	"github.com/go-spring/spring-core/web"
	"github.com/spf13/cast"
)

func init() {
	conf.NewScheme(k8s.Scheme, "k8s")
}

const SPRING_PROFILE = "SPRING_PROFILE"

type ApplicationContext interface {
	Go(fn func(ctx context.Context))
}

// ApplicationRunner 命令行启动器接口
type ApplicationRunner interface {
	Run(ctx ApplicationContext)
}

// ApplicationEvent 应用运行过程中的事件
type ApplicationEvent interface {
	OnStartApplication(ctx ApplicationContext) // 应用启动的事件
	OnStopApplication(ctx ApplicationContext)  // 应用停止的事件
}

// App 应用
type App struct {
	c *Container // 应用上下文

	cfgLocation         []string // 配置文件目录
	banner              string   // Banner 的内容
	bannerMode          int      // Banner 的显式模式
	expectSysProperties []string // 期望从系统环境变量中获取到的属性，支持正则表达式

	Events  []ApplicationEvent  `autowire:"${application-event.collection:=[]?}"`
	Runners []ApplicationRunner `autowire:"${command-line-runner.collection:=[]?}"`

	exitChan chan struct{}

	rootRouter  web.RootRouter
	consumers   map[string]*mq.BindConsumer
	gRPCServers map[string]*grpc.Service
}

// NewApp application 的构造函数
func NewApp() *App {
	return &App{
		c:                   New(),
		cfgLocation:         append([]string{}, "config/"),
		bannerMode:          BannerModeConsole,
		expectSysProperties: []string{`.*`},
		exitChan:            make(chan struct{}),
		rootRouter:          web.NewRootRouter(),
		consumers:           make(map[string]*mq.BindConsumer),
		gRPCServers:         make(map[string]*grpc.Service),
	}
}

// Start 启动应用
func (app *App) start(cfgLocation ...string) {

	if len(cfgLocation) > 0 {
		app.cfgLocation = cfgLocation
	}

	// 打印 Banner 内容
	if app.bannerMode != BannerModeOff {
		printBanner(app.getBanner())
	}

	app.Object(app)
	app.prepare()

	openPandora := app.c.p.Get("spring.application.open-pandora")
	if cast.ToBool(openPandora) {
		app.Object(&pandora{app.c}).Export((*Pandora)(nil))
	}

	app.c.Refresh()

	// 执行命令行启动器
	for _, r := range app.Runners {
		r.Run(app)
	}

	// 通知应用启动事件
	for _, e := range app.Events {
		e.OnStartApplication(app)
	}

	log.Info("application started successfully")
}

func (app *App) getBanner() string {

	if len(app.banner) > 0 {
		return app.banner
	}

	for _, dir := range app.cfgLocation {
		stat, err := os.Stat(dir)
		if err != nil || !stat.IsDir() {
			continue
		}

		f := path.Join(dir, "banner.txt")
		stat, err = os.Stat(f)
		if err != nil || stat.IsDir() {
			continue
		}

		b, err := ioutil.ReadFile(f)
		if err == nil {
			return string(b)
		}
	}

	return defaultBanner
}

// loadCmdArgs 加载命令行参数，形如 -name value 的参数才有效。
func (app *App) loadCmdArgs(p *conf.Properties) {
	log.Debugf("load cmd args")
	for i := 0; i < len(os.Args); i++ { // 以短线定义的参数才有效
		a := os.Args[i]
		if !strings.HasPrefix(a, "-") {
			continue
		}
		k, v := a[1:], ""
		if i < len(os.Args)-1 && !strings.HasPrefix(os.Args[i+1], "-") {
			v = os.Args[i+1]
			i++
		}
		log.Tracef("%s=%v", k, v)
		p.Set(k, v)
	}
}

// loadSystemEnv 加载环境变量，用户可以使用正则表达式来提取想要的环境变量。
func (app *App) loadSystemEnv(p *conf.Properties) error {

	var rex []*regexp.Regexp
	for _, v := range app.expectSysProperties {
		exp, err := regexp.Compile(v)
		if err != nil {
			return err
		}
		rex = append(rex, exp)
	}

	log.Debugf("load system env")
	for _, env := range os.Environ() {
		i := strings.Index(env, "=")
		if i <= 0 {
			continue
		}
		k, v := env[0:i], env[i+1:]
		for _, r := range rex {
			if !r.MatchString(k) {
				continue
			}
			log.Tracef("%s=%v", k, v)
			p.Set(k, v)
			break
		}
	}
	return nil
}

// loadConfigFile 加载指定环境的配置文件
func (app *App) loadConfigFile(p *conf.Properties, profile ...string) error {

	fileName := "application"
	if len(profile) > 0 && profile[0] != "" {
		fileName += "-" + profile[0]
	}

	var (
		schemeName   string
		fileLocation string
	)

	configTypes := []string{"properties", "yaml", "toml"}
	for _, configLocation := range app.cfgLocation {

		ss := strings.SplitN(configLocation, ":", 2)
		if len(ss) == 1 {
			fileLocation = ss[0]
		} else {
			schemeName = ss[0]
			fileLocation = ss[1]
		}

		scheme, ok := conf.FindScheme(schemeName)
		if !ok {
			panic(fmt.Errorf("unsupported config scheme %s", schemeName))
		}

		err := scheme(p, fileLocation, fileName, configTypes)
		if err != nil {
			return err
		}

		// TODO Trace 打印所有的属性。
	}

	return nil
}

// prepare 准备上下文环境
func (app *App) prepare() {

	// 配置项加载顺序优先级，从高到低:
	// 1.代码设置(api)
	// 2.命令行参数
	// 3.系统环境变量
	// 4.application-profile.conf
	// 5.application.conf
	// 6.内部默认配置

	cmdConfig := conf.New()
	envConfig := conf.New()
	profileConfig := conf.New()
	defaultConfig := conf.New()

	p := []*conf.Properties{
		app.c.p,
		cmdConfig,
		envConfig,
		profileConfig,
		defaultConfig,
	}

	app.loadCmdArgs(cmdConfig)

	err := app.loadSystemEnv(envConfig)
	util.Panic(err).When(err != nil)

	err = app.loadConfigFile(defaultConfig)
	util.Panic(err).When(err != nil)

	profile := func([]*conf.Properties) string {
		keys := []string{conf.SpringProfile, SPRING_PROFILE}
		for _, c := range p {
			for _, k := range keys {
				v := c.Get(k, conf.DisableResolve())
				if v != nil {
					return cast.ToString(v)
				}
			}
		}
		return ""
	}(p)

	if profile != "" {
		err = app.loadConfigFile(profileConfig, profile)
		util.Panic(err).When(err != nil)
		app.c.Property(conf.SpringProfile, profile)
	}

	m := make(map[string]interface{})
	for _, c := range p {
		for k, v := range util.FlatMap(c.Map()) {
			if _, ok := m[k]; !ok {
				m[k] = v
			}
		}
	}

	// 将重组后的属性值写入 Context 属性列表
	for key, val := range m {
		app.c.Property(key, val)
	}
}

func (app *App) close() {

	defer log.Info("application exited")
	log.Info("application exiting")

	// OnStopApplication 是否需要有 Timeout 的 Context？
	// 仔细想想没有必要，程序想要优雅退出就得一直等，等到所有工作
	// 做完，用户如果等不急了可以使用 kill -9 进行硬杀，也就是
	// 是否优雅退出取决于用户。这样的话，OnStopApplication 不
	// 依赖 appCtx 的 Context，就只需要考虑 SafeGoroutine
	// 的退出了，而这只需要 Context 一 cancel 也就完事了。

	app.Go(func(ctx context.Context) {
		select {
		case <-ctx.Done():
			for _, b := range app.Events {
				b.OnStopApplication(app)
			}
		}
	})

	app.c.Close()
}

func (app *App) Run(cfgLocation ...string) {

	// 响应控制台的 Ctrl+C 及 kill 命令。
	go func() {
		ch := make(chan os.Signal, 1)
		signal.Notify(ch, os.Interrupt, syscall.SIGTERM)
		sig := <-ch
		log.Infof("program will exit because of signal %v", sig)
		app.ShutDown()
	}()

	app.start(cfgLocation...)
	<-app.exitChan
	app.close()
}

// ShutDown 关闭执行器
func (app *App) ShutDown() {
	select {
	case <-app.exitChan:
		// chan 已关闭，无需再次关闭。
	default:
		close(app.exitChan)
	}
}

func (app *App) ExpectSysProperties(pattern ...string) {
	app.expectSysProperties = pattern
}

func (app *App) BannerMode(mode int) {
	app.bannerMode = mode
}

// Banner 设置自定义 Banner 字符串
func (app *App) Banner(banner string) {
	app.banner = banner
}

func (app *App) Property(key string, value interface{}) {
	app.c.Property(key, value)
}

func (app *App) Object(i interface{}) *BeanDefinition {
	return app.c.register(NewBean(reflect.ValueOf(i)))
}

func (app *App) Provide(ctor interface{}, args ...arg.Arg) *BeanDefinition {
	return app.c.register(NewBean(ctor, args...))
}

func (app *App) Config(fn interface{}, args ...arg.Arg) *Configer {
	return app.c.config(NewConfiger(fn, args...))
}

func (app *App) Go(fn func(ctx context.Context)) {
	app.c.Go(fn)
}

func (app *App) GRPCServers() map[string]*grpc.Service {
	return app.gRPCServers
}

// GRpcServer 注册 gRPC 服务提供者，fn 是 gRPC 自动生成的服务注册函数，serviceName 是服务名称，
// 必须对应 *_grpc.pg.go 文件里面 grpc.ServiceDesc 的 ServiceName 字段，server 是服务具体提供者对象。
func (app *App) GRpcServer(serviceName string, fn interface{}, server interface{}) {
	s := &grpc.Service{Register: fn, Server: server}
	app.gRPCServers[serviceName] = s
}

// GRpcClient 注册 gRPC 服务客户端，fn 是 gRPC 自动生成的客户端构造函数
func (app *App) GRpcClient(fn interface{}, endpoint string) *BeanDefinition {
	return app.c.register(NewBean(fn, endpoint))
}

func (app *App) Mappers() map[string]*web.Mapper {
	return app.rootRouter.Mappers()
}

// Route 返回和 Mapping 绑定的路由分组
func (app *App) Route(basePath string) *web.Router {
	return app.rootRouter.Route(basePath)
}

// HandleRequest 注册任意 HTTP 方法处理函数
func (app *App) HandleRequest(method uint32, path string, fn web.Handler) *web.Mapper {
	return app.rootRouter.HandleRequest(method, path, fn)
}

// RequestMapping 注册任意 HTTP 方法处理函数
func (app *App) RequestMapping(method uint32, path string, fn web.HandlerFunc) *web.Mapper {
	return app.rootRouter.RequestMapping(method, path, fn)
}

// RequestBinding 注册任意 HTTP 方法处理函数
func (app *App) RequestBinding(method uint32, path string, fn interface{}) *web.Mapper {
	return app.rootRouter.RequestBinding(method, path, fn)
}

// HandleGet 注册 GET 方法处理函数
func (app *App) HandleGet(path string, fn web.Handler) *web.Mapper {
	return app.rootRouter.HandleGet(path, fn)
}

// GetMapping 注册 GET 方法处理函数
func (app *App) GetMapping(path string, fn web.HandlerFunc) *web.Mapper {
	return app.rootRouter.GetMapping(path, fn)
}

// GetBinding 注册 GET 方法处理函数
func (app *App) GetBinding(path string, fn interface{}) *web.Mapper {
	return app.rootRouter.GetBinding(path, fn)
}

// HandlePost 注册 POST 方法处理函数
func (app *App) HandlePost(path string, fn web.Handler) *web.Mapper {
	return app.rootRouter.HandlePost(path, fn)
}

// PostMapping 注册 POST 方法处理函数
func (app *App) PostMapping(path string, fn web.HandlerFunc) *web.Mapper {
	return app.rootRouter.PostMapping(path, fn)
}

// PostBinding 注册 POST 方法处理函数
func (app *App) PostBinding(path string, fn interface{}) *web.Mapper {
	return app.rootRouter.PostBinding(path, fn)
}

// HandlePut 注册 PUT 方法处理函数
func (app *App) HandlePut(path string, fn web.Handler) *web.Mapper {
	return app.rootRouter.HandlePut(path, fn)
}

// PutMapping 注册 PUT 方法处理函数
func (app *App) PutMapping(path string, fn web.HandlerFunc) *web.Mapper {
	return app.rootRouter.PutMapping(path, fn)
}

// PutBinding 注册 PUT 方法处理函数
func (app *App) PutBinding(path string, fn interface{}) *web.Mapper {
	return app.rootRouter.PutBinding(path, fn)
}

// HandleDelete 注册 DELETE 方法处理函数
func (app *App) HandleDelete(path string, fn web.Handler) *web.Mapper {
	return app.rootRouter.HandleDelete(path, fn)
}

// DeleteMapping 注册 DELETE 方法处理函数
func (app *App) DeleteMapping(path string, fn web.HandlerFunc) *web.Mapper {
	return app.rootRouter.DeleteMapping(path, fn)
}

// DeleteBinding 注册 DELETE 方法处理函数
func (app *App) DeleteBinding(path string, fn interface{}) *web.Mapper {
	return app.rootRouter.DeleteBinding(path, web.BIND(fn))
}

func (app *App) NewFilter(objOrCtor interface{}, ctorArgs ...arg.Arg) *BeanDefinition {
	b := NewBean(objOrCtor, ctorArgs...)
	return app.c.register(b).Export((*web.Filter)(nil))
}

func (app *App) Consumers() map[string]*mq.BindConsumer {
	return app.consumers
}

func (app *App) Consume(topic string, fn interface{}) {
	app.consumers[topic] = mq.BIND(topic, fn)
}
