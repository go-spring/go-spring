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

package app

import (
	"context"
	"flag"
	"fmt"
	"io/ioutil"
	"os"
	"os/signal"
	"path"
	"regexp"
	"strings"
	"syscall"

	"github.com/go-spring/spring-core/core"
	"github.com/go-spring/spring-core/log"
	"github.com/spf13/cast"
)

const (
	DefaultConfigLocation = "config/" // 默认的配置文件路径
)

const (
	SpringProfile  = "spring.profile" // 运行环境
	SPRING_PROFILE = "SPRING_PROFILE"
)

var (
	_ = flag.String(SpringProfile, "", "设置运行环境")
)

// CommandLineRunner 命令行启动器接口
type CommandLineRunner interface {
	Run(ctx core.ApplicationContext)
}

// ApplicationEvent 应用运行过程中的事件
type ApplicationEvent interface {
	OnStartApplication(ctx core.ApplicationContext) // 应用启动的事件
	OnStopApplication(ctx core.ApplicationContext)  // 应用停止的事件
}

// AfterPrepareFunc 定义 app.prepare() 执行完成之后的扩展点
type AfterPrepareFunc func(ctx core.ApplicationContext)

// application 应用
type application struct {
	appCtx core.ApplicationContext // 应用上下文

	cfgLocation         []string           // 配置文件目录
	banner              string             // Banner 的内容
	bannerMode          BannerMode         // Banner 的显式模式
	expectSysProperties []string           // 期望从系统环境变量中获取到的属性，支持正则表达式
	listOfAfterPrepare  []AfterPrepareFunc // app.prepare() 执行完成之后的扩展点的集合

	Events  []ApplicationEvent  `autowire:"${application-event.collection:=[]?}"`
	Runners []CommandLineRunner `autowire:"${command-line-runner.collection:=[]?}"`

	exitChan chan struct{}
}

var gApp = New()

// New application 的构造函数
func New() *application {
	return &application{
		appCtx:              core.NewApplicationContext(),
		cfgLocation:         append([]string{}, DefaultConfigLocation),
		bannerMode:          BannerModeConsole,
		expectSysProperties: []string{`.*`},
		exitChan:            make(chan struct{}),
	}
}

// Start 启动应用
func (app *application) start(cfgLocation ...string) {

	app.cfgLocation = append(app.cfgLocation, cfgLocation...)

	// 打印 Banner 内容
	if app.bannerMode != BannerModeOff {
		app.printBanner()
	}

	// 准备上下文环境
	app.prepare()

	// 执行所有 app.prepare() 之后执行的扩展点
	for _, fn := range app.listOfAfterPrepare {
		fn(app.appCtx)
	}

	// 注册 ApplicationContext 接口
	app.appCtx.ObjBean(app.appCtx).Export((*core.ApplicationContext)(nil))

	// 依赖注入、属性绑定、初始化
	app.appCtx.AutoWireBeans()

	// 执行命令行启动器
	for _, r := range app.Runners {
		r.Run(app.appCtx)
	}

	// 通知应用启动事件
	for _, b := range app.Events {
		b.OnStartApplication(app.appCtx)
	}

	log.Info("application started")
}

// printBanner 查找 Banner 文件然后将其打印到控制台
func (app *application) printBanner() {

	// 优先使用自定义 Banner
	banner := app.banner

	// 然后是文件中的 Banner
	if banner == "" {
		for _, configLocation := range app.cfgLocation {
			if stat, err := os.Stat(configLocation); err == nil && stat.IsDir() {
				f := path.Join(configLocation, "banner.txt")
				if stat, err = os.Stat(f); err == nil && !stat.IsDir() {
					if s, e := ioutil.ReadFile(f); e == nil {
						banner = string(s)
						break
					} else {
						panic(e)
					}
				}
			}
		}
	}

	// 最后是默认的 Banner
	if banner == "" {
		banner = defaultBanner
	}

	printBanner(banner)
}

// loadCmdArgs 加载命令行参数，形如 -name value 的参数才有效。
func (_ *application) loadCmdArgs() core.Properties {
	log.Debugf("load cmd args")
	p := core.New()
	for i := 0; i < len(os.Args); i++ { // 以短线定义的参数才有效
		if arg := os.Args[i]; strings.HasPrefix(arg, "-") {
			k, v := arg[1:], ""
			if i < len(os.Args)-1 && !strings.HasPrefix(os.Args[i+1], "-") {
				v = os.Args[i+1]
				i++
			}
			log.Tracef("%s=%v", k, v)
			p.Set(k, v)
		}
	}
	return p
}

// loadSystemEnv 加载系统环境变量，用户可以自定义有效环境变量的正则匹配
func (app *application) loadSystemEnv() core.Properties {

	var rex []*regexp.Regexp
	for _, v := range app.expectSysProperties {
		if exp, err := regexp.Compile(v); err != nil {
			panic(err)
		} else {
			rex = append(rex, exp)
		}
	}

	log.Debugf("load system env")
	p := core.New()
	for _, env := range os.Environ() {
		if i := strings.Index(env, "="); i > 0 {
			k, v := env[0:i], env[i+1:]
			for _, r := range rex {
				if r.MatchString(k) { // 符合匹配规则的才有效
					log.Tracef("%s=%v", k, v)
					p.Set(k, v)
					break
				}
			}
		}
	}
	return p
}

// loadProfileConfig 加载指定环境的配置文件
func (app *application) loadProfileConfig(profile string) core.Properties {
	p := core.New()
	for _, configLocation := range app.cfgLocation {
		var result map[string]interface{}
		if ss := strings.SplitN(configLocation, ":", 2); len(ss) == 1 {
			result = new(defaultPropertySource).Load(ss[0], profile)
		} else {
			if ps, ok := propertySources[ss[0]]; ok {
				result = ps.Load(ss[1], profile)
			} else {
				panic(fmt.Errorf("unsupported config scheme %s", ss[0]))
			}
		}
		for k, v := range result {
			log.Tracef("%s=%v", k, v)
			p.Set(k, v)
		}
	}
	return p
}

// resolveProperty 解析属性值，查看其是否具有引用关系
func (app *application) resolveProperty(conf map[string]interface{}, key string, value interface{}) interface{} {
	if s, o := value.(string); o && strings.HasPrefix(s, "${") {
		refKey := s[2 : len(s)-1]
		if refValue, ok := conf[refKey]; !ok {
			panic(fmt.Errorf("property \"%s\" not config", refKey))
		} else {
			refValue = app.resolveProperty(conf, refKey, refValue)
			conf[key] = refValue
			return refValue
		}
	}
	return value
}

// prepare 准备上下文环境
func (app *application) prepare() {

	// 配置项加载顺序优先级，从高到低:
	// 1.代码设置
	// 2.命令行参数
	// 3.系统环境变量
	// 4.application-profile.conf
	// 5.application.conf
	// 6.内部默认配置

	// 将通过代码设置的属性值拷贝一份，第 1 层
	apiConfig := core.New()
	app.appCtx.Properties().Range(func(k string, v interface{}) { apiConfig.Set(k, v) })

	// 加载默认的应用配置文件，如 application.conf，第 5 层
	appConfig := app.loadProfileConfig("")
	p := core.Priority(apiConfig, appConfig)

	// 加载系统环境变量，第 3 层
	sysEnv := app.loadSystemEnv()
	p.InsertBefore(sysEnv, appConfig)

	// 加载命令行参数，第 2 层
	cmdArgs := app.loadCmdArgs()
	p.InsertBefore(cmdArgs, sysEnv)

	// 加载特定环境的配置文件，如 application-test.conf
	profile := app.appCtx.GetProfile()
	if profile == "" {
		keys := []string{SpringProfile, SPRING_PROFILE}
		profile = cast.ToString(p.GetFirst(keys...))
	}
	if profile != "" {
		app.appCtx.SetProfile(profile) // 第 4 层
		profileConfig := app.loadProfileConfig(profile)
		p.InsertBefore(profileConfig, appConfig)
	}

	properties := map[string]interface{}{}
	p.Fill(properties)

	// 将重组后的属性值写入 ApplicationContext 属性列表
	for key, value := range properties {
		value = app.resolveProperty(properties, key, value)
		app.appCtx.SetProperty(key, value)
	}
}

func (app *application) close() {

	defer log.Info("application exited")
	log.Info("application exiting")

	// OnStopApplication 是否需要有 Timeout 的 Context？
	// 仔细想想没有必要，程序想要优雅退出就得一直等，等到所有工作
	// 做完，用户如果等不急了可以使用 kill -9 进行硬杀，也就是
	// 是否优雅退出取决于用户。这样的话，OnStopApplication 不
	// 依赖 appCtx 的 Context，就只需要考虑 SafeGoroutine
	// 的退出了，而这只需要 Context 一 cancel 也就完事了。

	// 通知 Bean 销毁
	app.appCtx.Close(func() {
		for _, b := range app.Events {
			b.OnStopApplication(app.appCtx)
		}
	})
}

func (app *application) ApplicationContext() core.ApplicationContext {
	return app.appCtx
}

// WithBannerMode 设置 Banner 的显式模式
func WithBannerMode(mode BannerMode) {
	gApp.WithBannerMode(mode)
}

func (app *application) WithBannerMode(mode BannerMode) *application {
	app.bannerMode = mode
	return app
}

// AfterPrepare 注册一个 gApp.prepare() 执行完成之后的扩展点
func AfterPrepare(fn AfterPrepareFunc) {
	gApp.AfterPrepare(fn)
}

func (app *application) AfterPrepare(fn AfterPrepareFunc) *application {
	app.listOfAfterPrepare = append(app.listOfAfterPrepare, fn)
	return app
}

// ExpectSysProperties 期望从系统环境变量中获取到的属性，支持正则表达式
func ExpectSysProperties(pattern ...string) {
	gApp.ExpectSysProperties(pattern...)
}

func (app *application) ExpectSysProperties(pattern ...string) *application {
	app.expectSysProperties = pattern
	return app
}

// Bean 注册 bean.BeanDefinition 对象。
func (app *application) Bean(bd *core.BeanDefinition) *application {
	app.appCtx.RegisterBean(bd)
	return app
}

// GetProfile 返回运行环境
func GetProfile() string {
	return gApp.ApplicationContext().GetProfile()
}

// Profile 设置运行环境
func Profile(profile string) {
	gApp.Profile(profile)
}

// Profile 设置运行环境
func (app *application) Profile(profile string) *application {
	gApp.ApplicationContext().SetProfile(profile)
	return app
}

// ObjBean 注册单例 Bean，不指定名称，重复注册会 panic。
func ObjBean(i interface{}) *core.BeanDefinition {
	bd := core.ObjBean(i)
	gApp.Bean(bd)
	return bd
}

// CtorBean 注册单例构造函数 Bean，不指定名称，重复注册会 panic。
func CtorBean(fn interface{}, args ...core.Arg) *core.BeanDefinition {
	bd := core.CtorBean(fn, args...)
	gApp.Bean(bd)
	return bd
}

// MethodBean 注册成员方法单例 Bean，不指定名称，重复注册会 panic。
// 必须给定方法名而不能通过遍历方法列表比较方法类型的方式获得函数名，因为不同方法的类型可能相同。
// 而且 interface 的方法类型不带 receiver 而成员方法的类型带有 receiver，两者类型也不好匹配。
func MethodBean(selector core.BeanSelector, method string, args ...core.Arg) *core.BeanDefinition {
	bd := core.MethodBean(selector, method, args...)
	gApp.Bean(bd)
	return bd
}

// WireBean 对外部的 Bean 进行依赖注入和属性绑定
func WireBean(i interface{}) {
	gApp.ApplicationContext().WireBean(i)
}

// GetBean 获取单例 Bean，若多于 1 个则 panic；找到返回 true 否则返回 false。
// 它和 FindBean 的区别是它在调用后能够保证返回的 Bean 已经完成了注入和绑定过程。
func GetBean(i interface{}, selector ...core.BeanSelector) bool {
	return gApp.ApplicationContext().GetBean(i, selector...)
}

// FindBean 查询单例 Bean，若多于 1 个则 panic；找到返回 true 否则返回 false。
// 它和 GetBean 的区别是它在调用后不能保证返回的 Bean 已经完成了注入和绑定过程。
func FindBean(selector core.BeanSelector) (*core.BeanDefinition, bool) {
	return gApp.ApplicationContext().FindBean(selector)
}

// CollectBeans 收集数组或指针定义的所有符合条件的 Bean，收集到返回 true，否则返
// 回 false。该函数有两种模式:自动模式和指定模式。自动模式是指 selectors 参数为空，
// 这时候不仅会收集符合条件的单例 Bean，还会收集符合条件的数组 Bean (是指数组的元素
// 符合条件，然后把数组元素拆开一个个放到收集结果里面)。指定模式是指 selectors 参数
// 不为空，这时候只会收集单例 Bean，而且要求这些单例 Bean 不仅需要满足收集条件，而且
// 必须满足 selector 条件。另外，自动模式下不对收集结果进行排序，指定模式下根据
// selectors 列表的顺序对收集结果进行排序。
func CollectBeans(i interface{}, selectors ...core.BeanSelector) bool {
	return gApp.ApplicationContext().CollectBeans(i, selectors...)
}

// GetBeanDefinitions 获取所有 Bean 的定义，不能保证解析和注入，请谨慎使用该函数!
func GetBeanDefinitions() []*core.BeanDefinition {
	return gApp.ApplicationContext().GetBeanDefinitions()
}

// GetProperty 返回属性值，不能存在返回 nil，属性名称统一转成小写。
func GetProperty(key string) interface{} {
	return gApp.ApplicationContext().GetProperty(key)
}

// Property 设置属性值，属性名称统一转成小写。
func Property(key string, value interface{}) {
	gApp.Property(key, value)
}

// Property 设置属性值，属性名称统一转成小写。
func (app *application) Property(key string, value interface{}) *application {
	app.appCtx.SetProperty(key, value)
	return app
}

// Invoke 立即执行一个一次性的任务
func Invoke(fn interface{}, args ...core.Arg) error {
	return gApp.ApplicationContext().Invoke(fn, args...)
}

// Config 注册一个配置函数
func Config(fn interface{}, args ...core.Arg) *core.Configer {
	configer := core.Config(fn, args...)
	gApp.Config(configer)
	return configer
}

// Config 注册一个配置函数
func (app *application) Config(configer *core.Configer) *application {
	app.ApplicationContext().WithConfig(configer)
	return app
}

type GoFuncWithContext func(context.Context)

// Go 安全地启动一个 goroutine
func Go(fn GoFuncWithContext) {
	gApp.ApplicationContext().Go(func() { fn(gApp.ApplicationContext().Context()) })
}

// Run 快速启动 boot 应用
func Run(cfgLocation ...string) {
	gApp.Run(cfgLocation...)
}

func (app *application) Run(cfgLocation ...string) {

	// 响应控制台的 Ctrl+C 及 kill 命令。
	go func() {
		sig := make(chan os.Signal, 1)
		signal.Notify(sig, os.Interrupt, syscall.SIGTERM)
		<-sig
		fmt.Println("got signal, program will exit")
		app.ShutDown()
	}()

	app.start(cfgLocation...)
	<-app.exitChan
	app.close()
}

// ShutDown 退出 boot 应用
func ShutDown() {
	gApp.ShutDown()
}

// ShutDown 关闭执行器
func (app *application) ShutDown() {
	select {
	case <-app.exitChan:
		// chan 已关闭，无需再次关闭。
	default:
		close(app.exitChan)
	}
}
