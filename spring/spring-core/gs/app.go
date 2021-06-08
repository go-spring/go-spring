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
	"container/list"
	"context"
	"io/ioutil"
	"os"
	"os/signal"
	"path"
	"path/filepath"
	"reflect"
	"regexp"
	"strings"
	"syscall"

	"github.com/go-spring/spring-core/arg"
	"github.com/go-spring/spring-core/bean"
	"github.com/go-spring/spring-core/cond"
	"github.com/go-spring/spring-core/conf"
	"github.com/go-spring/spring-core/grpc"
	"github.com/go-spring/spring-core/log"
	"github.com/go-spring/spring-core/mq"
	"github.com/go-spring/spring-core/sort"
	"github.com/go-spring/spring-core/util"
	"github.com/go-spring/spring-core/web"
	"github.com/spf13/cast"
)

const SPRING_PROFILE = "SPRING_PROFILE"

type PropertySource func(app, profile, ext string) (*conf.Properties, error)

type BootstrapContext interface {
	Prop(key string, opts ...conf.GetOption) interface{}
	AddPropertySource(ps PropertySource)
}

type Bootstrap func(BootstrapContext)

type bootstrapContext struct {
	p *conf.Properties
	s []PropertySource
}

func (ctx *bootstrapContext) Prop(key string, opts ...conf.GetOption) interface{} {
	return ctx.p.Get(key, opts...)
}

func (ctx *bootstrapContext) Find(selector bean.Selector) (bean.Definition, error) {
	panic(util.UnimplementedMethod)
}

func (ctx *bootstrapContext) AddPropertySource(ps PropertySource) {
	ctx.s = append(ctx.s, ps)
}

// loadCmdArgs 加载命令行参数，形如 -name value 的参数才有效。
func (ctx *bootstrapContext) loadCmdArgs() error {
	log.Debug("load cmd args")
	for i := 0; i < len(os.Args); i++ {
		a := os.Args[i]
		if !strings.HasPrefix(a, "-") {
			continue
		}
		k, v := a[1:], ""
		if i < len(os.Args)-1 && !strings.HasPrefix(os.Args[i+1], "-") {
			v = os.Args[i+1]
			i++
		}
		if ctx.p.Get(k, conf.DisableResolve()) == nil {
			log.Tracef("%v=%v", k, v)
			ctx.p.Set(k, v)
		}
	}
	return nil
}

// loadSystemEnv 加载环境变量，用户可以使用正则表达式来提取想要的环境变量。
func (ctx *bootstrapContext) loadSystemEnv(expectSystemEnv []string) error {
	log.Debug("load system env")

	var rex []*regexp.Regexp
	for _, v := range expectSystemEnv {
		exp, err := regexp.Compile(v)
		if err != nil {
			return err
		}
		rex = append(rex, exp)
	}

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
			if ctx.p.Get(k, conf.DisableResolve()) == nil {
				log.Tracef("%v=%v", k, v)
				ctx.p.Set(k, v)
			}
			break
		}
	}
	return nil
}

type BootstrapDefinition struct {
	b      Bootstrap
	name   string
	cond   cond.Condition
	before []string // 位于哪些配置函数之前
	after  []string // 位于哪些配置函数之后
}

// WithName 为 Bootstrap 设置一个名称。
func (d *BootstrapDefinition) WithName(name string) *BootstrapDefinition {
	d.name = name
	return d
}

// WithCond 为 Bootstrap 设置一个 Condition。
func (d *BootstrapDefinition) WithCond(cond cond.Condition) *BootstrapDefinition {
	d.cond = cond
	return d
}

// Before 设置当前 Bootstrap 在哪些 Bootstrap 之前执行。
func (d *BootstrapDefinition) Before(bootstraps ...string) *BootstrapDefinition {
	d.before = append(d.before, bootstraps...)
	return d
}

// After 设置当前 Bootstrap 在哪些 Bootstrap 之后执行。
func (d *BootstrapDefinition) After(bootstraps ...string) *BootstrapDefinition {
	d.after = append(d.after, bootstraps...)
	return d
}

// getBeforeBootstraps 获取 i 之前的 Bootstrap 列表，用于 sort.Triple 排序。
func getBeforeBootstraps(bootstraps *list.List, i interface{}) *list.List {

	result := list.New()
	current := i.(*BootstrapDefinition)
	for e := bootstraps.Front(); e != nil; e = e.Next() {
		c := e.Value.(*BootstrapDefinition)

		// 检查 c 是否在 current 的前面
		for _, name := range c.before {
			if current.name == name {
				result.PushBack(c)
			}
		}

		// 检查 current 是否在 c 的后面
		for _, name := range current.after {
			if c.name == name {
				result.PushBack(c)
			}
		}
	}
	return result
}

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

	banner     string // banner 的内容
	showBanner bool   // 是否显示 banner

	bootstrapList *list.List

	configLocations []string // 属性列表文件的读取地址
	configTypeOrder []string // 属性列表文件的读取顺序
	expectSystemEnv []string // 获取环境变量的正则表达式

	Events  []ApplicationEvent  `autowire:"${application-event.collection:=[]?}"`
	Runners []ApplicationRunner `autowire:"${command-line-runner.collection:=[]?}"`

	exitChan chan struct{}

	RootRouter  web.RootRouter
	GRPCServers map[string]*grpc.Server
	Consumers   map[string]*mq.BindConsumer
}

// NewApp application 的构造函数
func NewApp() *App {
	return &App{
		c:               New(),
		showBanner:      true,
		bootstrapList:   list.New(),
		configLocations: []string{"config/"},
		configTypeOrder: []string{".properties", ".yaml", ".toml"},
		expectSystemEnv: []string{`.*`},
		exitChan:        make(chan struct{}),
		RootRouter:      web.NewRootRouter(),
		GRPCServers:     make(map[string]*grpc.Server),
		Consumers:       make(map[string]*mq.BindConsumer),
	}
}

// getBanner 获取 banner 字符串。
func (app *App) getBanner() string {

	if len(app.banner) > 0 {
		return app.banner
	}

	for _, location := range app.configLocations {

		stat, err := os.Stat(location)
		if err != nil || !stat.IsDir() {
			continue
		}

		file := path.Join(location, "banner.txt")
		b, err := ioutil.ReadFile(file)
		if err != nil {
			continue
		}

		return string(b)
	}
	return defaultBanner
}

// loadConfigFile 加载 profile 对应的属性列表文件。
func (app *App) loadConfigFile(s []PropertySource, profile string) (*conf.Properties, error) {
	ret := conf.New()
	for _, source := range s {
		for _, ext := range app.configTypeOrder {
			p, err := source("application", profile, ext)
			if err != nil {
				return nil, err
			}
			for k, v := range p.Map() {
				ret.Set(k, v)
			}
		}
	}
	return ret, nil
}

func (app *App) prepare() {

	// 属性列表加载顺序优先级，从高到低:
	// 1.API 设置
	// 2.命令行参数
	// 3.系统环境变量
	// 4.application-profile.conf
	// 5.application.conf
	// 6.属性绑定声明时的默认值

	defaultPS := func(prefix, profile, ext string) (*conf.Properties, error) {
		ret := conf.New()
		filename := prefix
		if profile != "" {
			filename += "-" + profile
		}
		filename += ext
		for _, location := range app.configLocations {
			p, err := conf.Load(filepath.Join(location, filename))
			if err != nil {
				if os.IsNotExist(err) {
					continue
				}
				return nil, err
			}
			for k, v := range p.Map() {
				if ret.Get(k, conf.DisableResolve()) == nil {
					log.Tracef("%v=%v", k, v)
					ret.Set(k, v)
				}
			}
		}
		return ret, nil
	}

	ctx := &bootstrapContext{p: conf.New()}
	ctx.s = append(ctx.s, defaultPS)

	for k, v := range app.c.p.Map() {
		ctx.p.Set(k, v)
	}

	err := ctx.loadCmdArgs()
	util.Panic(err).When(err != nil)

	err = ctx.loadSystemEnv(app.expectSystemEnv)
	util.Panic(err).When(err != nil)

	profile := func() string {
		keys := []string{conf.SpringProfile, SPRING_PROFILE}
		for _, k := range keys {
			v := ctx.p.Get(k, conf.DisableResolve())
			if v != nil {
				return cast.ToString(v)
			}
		}
		return ""
	}()

	if profile != "" {
		ctx.p.Set(conf.SpringProfile, profile)
	}

	sorted := sort.Triple(app.bootstrapList, getBeforeBootstraps)
	for e := sorted.Front(); e != nil; e = e.Next() {
		d := e.Value.(*BootstrapDefinition)
		if d.cond != nil {
			var ok bool
			ok, err = d.cond.Matches(ctx)
			util.Panic(err).When(err != nil)
			if !ok {
				continue
			}
		}
		d.b(ctx)
	}

	defaultConfig, err := app.loadConfigFile(ctx.s, "")
	util.Panic(err).When(err != nil)

	profileConfig, err := app.loadConfigFile(ctx.s, profile)
	util.Panic(err).When(err != nil)

	p := []*conf.Properties{
		ctx.p,
		defaultConfig,
		profileConfig,
	}

	m := make(map[string]interface{})
	for _, c := range p {
		for k, v := range util.FlatMap(c.Map()) {
			if _, ok := m[k]; !ok {
				m[k] = v
			}
		}
	}

	for key, val := range m {
		app.c.Property(key, val)
	}
}

func (app *App) start() {

	// 打印 Banner 内容
	if app.showBanner {
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

func (app *App) Run() {

	// 响应控制台的 Ctrl+C 及 kill 命令。
	go func() {
		ch := make(chan os.Signal, 1)
		signal.Notify(ch, os.Interrupt, syscall.SIGTERM)
		sig := <-ch
		log.Infof("program will exit because of signal %v", sig)
		app.ShutDown()
	}()

	app.start()
	<-app.exitChan
	app.close()
}

// ShutDown 关闭执行器
func (app *App) ShutDown() {
	select {
	case <-app.exitChan:
	default:
		close(app.exitChan)
	}
}

// Banner 自定义 banner 字符串。
func (app *App) Banner(banner string) {
	app.banner = banner
}

// ShowBanner 设置是否显示 banner。
func (app *App) ShowBanner(show bool) {
	app.showBanner = show
}

func (app *App) ExpectSystemEnv(pattern ...string) {
	app.expectSystemEnv = pattern
}

func (app *App) Bootstrap(b Bootstrap) *BootstrapDefinition {
	d := &BootstrapDefinition{b: b}
	app.bootstrapList.PushBack(d)
	return d
}

func (app *App) AddConfigLocation(cfgLocation ...string) {
	app.configLocations = append(app.configLocations, cfgLocation...)
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

// Route 返回和 Mapping 绑定的路由分组
func (app *App) Route(basePath string) *web.Router {
	return app.RootRouter.Route(basePath)
}

// HandleRequest 注册任意 HTTP 方法处理函数
func (app *App) HandleRequest(method uint32, path string, fn web.Handler) *web.Mapper {
	return app.RootRouter.HandleRequest(method, path, fn)
}

// RequestMapping 注册任意 HTTP 方法处理函数
func (app *App) RequestMapping(method uint32, path string, fn web.HandlerFunc) *web.Mapper {
	return app.RootRouter.RequestMapping(method, path, fn)
}

// RequestBinding 注册任意 HTTP 方法处理函数
func (app *App) RequestBinding(method uint32, path string, fn interface{}) *web.Mapper {
	return app.RootRouter.RequestBinding(method, path, fn)
}

// HandleGet 注册 GET 方法处理函数
func (app *App) HandleGet(path string, fn web.Handler) *web.Mapper {
	return app.RootRouter.HandleGet(path, fn)
}

// GetMapping 注册 GET 方法处理函数
func (app *App) GetMapping(path string, fn web.HandlerFunc) *web.Mapper {
	return app.RootRouter.GetMapping(path, fn)
}

// GetBinding 注册 GET 方法处理函数
func (app *App) GetBinding(path string, fn interface{}) *web.Mapper {
	return app.RootRouter.GetBinding(path, fn)
}

// HandlePost 注册 POST 方法处理函数
func (app *App) HandlePost(path string, fn web.Handler) *web.Mapper {
	return app.RootRouter.HandlePost(path, fn)
}

// PostMapping 注册 POST 方法处理函数
func (app *App) PostMapping(path string, fn web.HandlerFunc) *web.Mapper {
	return app.RootRouter.PostMapping(path, fn)
}

// PostBinding 注册 POST 方法处理函数
func (app *App) PostBinding(path string, fn interface{}) *web.Mapper {
	return app.RootRouter.PostBinding(path, fn)
}

// HandlePut 注册 PUT 方法处理函数
func (app *App) HandlePut(path string, fn web.Handler) *web.Mapper {
	return app.RootRouter.HandlePut(path, fn)
}

// PutMapping 注册 PUT 方法处理函数
func (app *App) PutMapping(path string, fn web.HandlerFunc) *web.Mapper {
	return app.RootRouter.PutMapping(path, fn)
}

// PutBinding 注册 PUT 方法处理函数
func (app *App) PutBinding(path string, fn interface{}) *web.Mapper {
	return app.RootRouter.PutBinding(path, fn)
}

// HandleDelete 注册 DELETE 方法处理函数
func (app *App) HandleDelete(path string, fn web.Handler) *web.Mapper {
	return app.RootRouter.HandleDelete(path, fn)
}

// DeleteMapping 注册 DELETE 方法处理函数
func (app *App) DeleteMapping(path string, fn web.HandlerFunc) *web.Mapper {
	return app.RootRouter.DeleteMapping(path, fn)
}

// DeleteBinding 注册 DELETE 方法处理函数
func (app *App) DeleteBinding(path string, fn interface{}) *web.Mapper {
	return app.RootRouter.DeleteBinding(path, web.BIND(fn))
}

// Filter 注册 web.Filter 对象。
func (app *App) Filter(objOrCtor interface{}, ctorArgs ...arg.Arg) *BeanDefinition {
	b := NewBean(objOrCtor, ctorArgs...)
	return app.c.register(b).Export((*web.Filter)(nil))
}

// Consume 注册 MQ 消费者。
func (app *App) Consume(topic string, fn interface{}) {
	app.Consumers[topic] = mq.BIND(topic, fn)
}

// GRPCClient 注册 gRPC 服务客户端，fn 是 gRPC 自动生成的客户端构造函数。
func (app *App) GRPCClient(fn interface{}, endpoint string) *BeanDefinition {
	return app.c.register(NewBean(fn, endpoint))
}

// GRPCServer 注册 gRPC 服务提供者，fn 是 gRPC 自动生成的服务注册函数，
// serviceName 是服务名称，必须对应 *_grpc.pg.go 文件里面 grpc.ServerDesc
// 的 ServiceName 字段，server 是服务提供者对象。
func (app *App) GRPCServer(serviceName string, fn interface{}, service interface{}) {
	s := &grpc.Server{Register: fn, Service: service}
	app.GRPCServers[serviceName] = s
}
