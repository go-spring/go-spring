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
	"regexp"
	"strings"
	"syscall"

	"github.com/go-spring/spring-boost/cast"
	"github.com/go-spring/spring-boost/errors"
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

	app.Object(app)
	app.Object(app.router)
	app.Object(app.consumers)

	e := &configuration{p: conf.New()}
	if err := e.prepare(); err != nil {
		return err
	}

	showBanner := cast.ToBool(e.p.Get(SpringBannerVisible))
	if showBanner {
		app.printBanner(app.getBanner(e.configLocations))
	}

	if app.b != nil {
		if err := app.bootstrap(e); err != nil {
			return err
		}
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

func (app *App) bootstrap(e *configuration) error {

	if err := app.b.start(e); err != nil {
		return err
	}

	sourceMap, err := app.b.sourceMap(e)
	if err != nil {
		return err
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
	if err := app.loadConfigFile(e, "application"); err != nil {
		return err
	}
	if e.activeProfile == "" {
		return nil
	}
	return app.loadConfigFile(e, "application-"+e.activeProfile)
}

func (app *App) loadConfigFile(e *configuration, filename string) error {
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
	if app.b == nil {
		app.b = &bootstrap{
			c:               New(),
			mapOfOnProperty: make(map[string]interface{}),
		}
	}
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

//////////////////////////// configuration ////////////////////////////

// EnvPrefix 属性覆盖的环境变量需要携带该前缀。
const EnvPrefix = "GS_"

// IncludeEnvPatterns 只加载符合条件的环境变量。
const IncludeEnvPatterns = "INCLUDE_ENV_PATTERNS"

// ExcludeEnvPatterns 排除符合条件的环境变量。
const ExcludeEnvPatterns = "EXCLUDE_ENV_PATTERNS"

// SpringProfilesActive 当前应用的 profile 配置。
const SpringProfilesActive = "spring.profiles.active"

// SpringConfigLocations 配置文件的位置，支持逗号分隔。
const SpringConfigLocations = "spring.config.locations"

// SpringConfigExtensions 配置文件的扩展名，支持逗号分隔。
const SpringConfigExtensions = "spring.config.extensions"

type Configuration interface {
	ActiveProfile() string
	ConfigLocations() []string
	ConfigExtensions() []string
	Properties() Properties
}

type configuration struct {
	p *conf.Properties

	activeProfile    string
	configLocations  []string
	configExtensions []string
}

func (e *configuration) ActiveProfile() string {
	return e.activeProfile
}

func (e *configuration) ConfigLocations() []string {
	return e.configLocations
}

func (e *configuration) ConfigExtensions() []string {
	return e.configExtensions
}

// loadCmdArgs 加载 -name value 形式的命令行参数。
func loadCmdArgs(p *conf.Properties) error {
	for i := 0; i < len(os.Args); i++ {

		s := os.Args[i]
		if !strings.HasPrefix(s, "-") {
			continue
		}

		k, v := s[1:], ""
		if i >= len(os.Args)-1 {
			p.Set(k, v)
			break
		}

		if !strings.HasPrefix(os.Args[i+1], "-") {
			v = os.Args[i+1]
			i++
		}
		p.Set(k, v)
	}
	return nil
}

// loadSystemEnv 添加符合 includes 条件的环境变量，排除符合 excludes 条件的
// 环境变量。如果发现存在允许通过环境变量覆盖的属性名，那么保存时转换成真正的属性名。
func loadSystemEnv(p *conf.Properties) error {

	toRex := func(patterns []string) ([]*regexp.Regexp, error) {
		var rex []*regexp.Regexp
		for _, v := range patterns {
			exp, err := regexp.Compile(v)
			if err != nil {
				return nil, err
			}
			rex = append(rex, exp)
		}
		return rex, nil
	}

	includes := []string{".*"}
	if s, ok := os.LookupEnv(IncludeEnvPatterns); ok {
		includes = strings.Split(s, ",")
	}
	includeRex, err := toRex(includes)
	if err != nil {
		return err
	}

	var excludes []string
	if s, ok := os.LookupEnv(ExcludeEnvPatterns); ok {
		excludes = strings.Split(s, ",")
	}
	excludeRex, err := toRex(excludes)
	if err != nil {
		return err
	}

	matches := func(rex []*regexp.Regexp, s string) bool {
		for _, r := range rex {
			if r.MatchString(s) {
				return true
			}
		}
		return false
	}

	for _, env := range os.Environ() {

		kv := strings.SplitN(env, "=", 2)
		if len(kv) == 1 {
			continue
		}

		k, v := kv[0], kv[1]
		if k == "" || v == "" {
			continue
		}

		if strings.HasPrefix(k, EnvPrefix) {
			propKey := strings.TrimPrefix(k, EnvPrefix)
			propKey = strings.ReplaceAll(propKey, "_", ".")
			p.Set(strings.ToLower(propKey), v)
			continue
		}

		if matches(excludeRex, k) || !matches(includeRex, k) {
			continue
		}
		p.Set(k, v)
	}
	return nil
}

func (e *configuration) prepare() error {

	if err := loadSystemEnv(e.p); err != nil {
		return err
	}

	if err := loadCmdArgs(e.p); err != nil {
		return err
	}

	s := e.p.Get(SpringConfigLocations, conf.Def("config/"))
	e.configLocations = strings.Split(cast.ToString(s), ",")

	extensions := ".properties,.prop,.yaml,.yml,.toml,.tml"
	s = e.p.Get(SpringConfigExtensions, conf.Def(extensions))
	e.configExtensions = strings.Split(cast.ToString(s), ",")

	e.activeProfile = cast.ToString(e.p.Get(SpringProfilesActive))
	return nil
}

func (e *configuration) Properties() Properties {
	return e.p
}

////////////////////////// bootstrap //////////////////////////

type PropertySource interface {
	Load(c Configuration) (map[string]Properties, error)
}

type bootstrap struct {

	// 应用上下文
	c *Container

	// 属性列表解析完成后的回调
	mapOfOnProperty map[string]interface{}

	PropertySources []PropertySource `autowire:""`
}

func validOnProperty(fn interface{}) error {
	t := reflect.TypeOf(fn)
	if t.Kind() != reflect.Func {
		return errors.New("fn should be a func(value_type)")
	}
	if t.NumIn() != 1 || !util.IsValueType(t.In(0)) || t.NumOut() != 0 {
		return errors.New("fn should be a func(value_type)")
	}
	return nil
}

// OnProperty 当 key 对应的属性值准备好后发送一个通知。
func (boot *bootstrap) OnProperty(key string, fn interface{}) {
	err := validOnProperty(fn)
	util.Panic(err).When(err != nil)
	boot.mapOfOnProperty[key] = fn
}

// Property 设置 key 对应的属性值，如果 key 对应的属性值已经存在则 Set 方法会
// 覆盖旧值。Set 方法除了支持 string 类型的属性值，还支持 int、uint、bool 等
// 其他基础数据类型的属性值。特殊情况下，Set 方法也支持 slice 、map 与基础数据
// 类型组合构成的属性值，其处理方式是将组合结构层层展开，可以将组合结构看成一棵树，
// 那么叶子结点的路径就是属性的 key，叶子结点的值就是属性的值。
func (boot *bootstrap) Property(key string, value interface{}) {
	boot.c.Property(key, value)
}

// Object 注册对象形式的 bean ，需要注意的是该方法在注入开始后就不能再调用了。
func (boot *bootstrap) Object(i interface{}) *BeanDefinition {
	return boot.c.register(NewBean(reflect.ValueOf(i)))
}

// Provide 注册构造函数形式的 bean ，需要注意的是该方法在注入开始后就不能再调用了。
func (boot *bootstrap) Provide(ctor interface{}, args ...arg.Arg) *BeanDefinition {
	return boot.c.register(NewBean(ctor, args...))
}

func (boot *bootstrap) start(e *configuration) error {

	boot.c.Object(boot)

	if err := boot.loadBootstrap(e); err != nil {
		return err
	}

	// 保存从环境变量和命令行解析的属性
	for _, k := range e.p.Keys() {
		boot.c.p.Set(k, e.p.Get(k))
	}

	for key, f := range boot.mapOfOnProperty {
		t := reflect.TypeOf(f)
		in := reflect.New(t.In(0)).Elem()
		err := boot.c.p.Bind(in, conf.Key(key))
		if err != nil {
			return err
		}
		reflect.ValueOf(f).Call([]reflect.Value{in})
	}

	return boot.c.Refresh()
}

func (boot *bootstrap) loadBootstrap(e *configuration) error {
	if err := boot.loadConfigFile(e, "bootstrap"); err != nil {
		return err
	}
	if e.activeProfile == "" {
		return nil
	}
	return boot.loadConfigFile(e, "bootstrap-"+e.activeProfile)
}

func (boot *bootstrap) loadConfigFile(e *configuration, filename string) error {
	for _, loc := range e.configLocations {
		for _, ext := range e.configExtensions {
			err := boot.c.Load(filepath.Join(loc, filename+ext))
			if err != nil && !os.IsNotExist(err) {
				return err
			}
		}
	}
	return nil
}

func (boot *bootstrap) sourceMap(e *configuration) (map[string][]Properties, error) {
	sourceMap := make(map[string][]Properties)
	for _, ps := range boot.PropertySources {
		m, err := ps.Load(e)
		if err != nil {
			return nil, err
		}
		for k, p := range m {
			sourceMap[k] = append(sourceMap[k], p)
		}
	}
	return sourceMap, nil
}
