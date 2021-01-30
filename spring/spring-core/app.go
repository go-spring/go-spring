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

package SpringCore

import (
	"flag"
	"fmt"
	"io/ioutil"
	"os"
	"path"
	"regexp"
	"strings"

	"github.com/go-spring/spring-logger"
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
	Run(ctx ApplicationContext)
}

// ApplicationEvent 应用运行过程中的事件
type ApplicationEvent interface {
	OnStartApplication(ctx ApplicationContext) // 应用启动的事件
	OnStopApplication(ctx ApplicationContext)  // 应用停止的事件
}

// AfterPrepareFunc 定义 app.prepare() 执行完成之后的扩展点
type AfterPrepareFunc func(ctx ApplicationContext)

// Application 应用
type Application struct {
	ApplicationContext // 应用上下文

	cfgLocation         []string           // 配置文件目录
	banner              string             // Banner 的内容
	bannerMode          BannerMode         // Banner 的显式模式
	expectSysProperties []string           // 期望从系统环境变量中获取到的属性，支持正则表达式
	listOfAfterPrepare  []AfterPrepareFunc // app.prepare() 执行完成之后的扩展点的集合

	Events  []ApplicationEvent  `autowire:"${application-event.collection:=[]?}"`
	Runners []CommandLineRunner `autowire:"${command-line-runner.collection:=[]?}"`
}

// NewApplication Application 的构造函数
func NewApplication() *Application {
	return &Application{
		ApplicationContext:  NewApplicationContext(),
		cfgLocation:         append([]string{}, DefaultConfigLocation),
		bannerMode:          BannerModeConsole,
		expectSysProperties: []string{`.*`},
	}
}

// AddConfigLocation 添加配置文件
func (app *Application) AddConfigLocation(cfgLocation ...string) *Application {
	app.cfgLocation = append(app.cfgLocation, cfgLocation...)
	return app
}

// SetBannerMode 设置 Banner 的显式模式
func (app *Application) SetBannerMode(mode BannerMode) *Application {
	app.bannerMode = mode
	return app
}

// ExpectSysProperties 期望从系统环境变量中获取到的属性，支持正则表达式
func (app *Application) ExpectSysProperties(pattern ...string) *Application {
	app.expectSysProperties = pattern
	return app
}

// AfterPrepare 注册一个 app.prepare() 执行完成之后的扩展点
func (app *Application) AfterPrepare(fn AfterPrepareFunc) *Application {
	app.listOfAfterPrepare = append(app.listOfAfterPrepare, fn)
	return app
}

// Start 启动应用
func (app *Application) Start() {

	// 打印 Banner 内容
	if app.bannerMode != BannerModeOff {
		app.printBanner()
	}

	// 准备上下文环境
	app.prepare()

	// 执行所有 app.prepare() 之后执行的扩展点
	for _, fn := range app.listOfAfterPrepare {
		fn(app)
	}

	// 注册 ApplicationContext 接口
	app.ObjBean(app).Export((*ApplicationContext)(nil))

	// 依赖注入、属性绑定、初始化
	app.AutoWireBeans()

	// 执行命令行启动器
	for _, r := range app.Runners {
		r.Run(app)
	}

	// 通知应用启动事件
	for _, bean := range app.Events {
		bean.OnStartApplication(app)
	}

	SpringLogger.Info("application started")
}

// printBanner 查找 Banner 文件然后将其打印到控制台
func (app *Application) printBanner() {

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
func (_ *Application) loadCmdArgs() Properties {
	SpringLogger.Debugf("load cmd args")
	p := NewDefaultProperties()
	for i := 0; i < len(os.Args); i++ { // 以短线定义的参数才有效
		if arg := os.Args[i]; strings.HasPrefix(arg, "-") {
			k, v := arg[1:], ""
			if i < len(os.Args)-1 && !strings.HasPrefix(os.Args[i+1], "-") {
				v = os.Args[i+1]
				i++
			}
			SpringLogger.Tracef("%s=%v", k, v)
			p.SetProperty(k, v)
		}
	}
	return p
}

// loadSystemEnv 加载系统环境变量，用户可以自定义有效环境变量的正则匹配
func (app *Application) loadSystemEnv() Properties {

	var rex []*regexp.Regexp
	for _, v := range app.expectSysProperties {
		if exp, err := regexp.Compile(v); err != nil {
			panic(err)
		} else {
			rex = append(rex, exp)
		}
	}

	SpringLogger.Debugf("load system env")
	p := NewDefaultProperties()
	for _, env := range os.Environ() {
		if i := strings.Index(env, "="); i > 0 {
			k, v := env[0:i], env[i+1:]
			for _, r := range rex {
				if r.MatchString(k) { // 符合匹配规则的才有效
					SpringLogger.Tracef("%s=%v", k, v)
					p.SetProperty(k, v)
					break
				}
			}
		}
	}
	return p
}

// loadProfileConfig 加载指定环境的配置文件
func (app *Application) loadProfileConfig(profile string) Properties {
	p := NewDefaultProperties()
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
			SpringLogger.Tracef("%s=%v", k, v)
			p.SetProperty(k, v)
		}
	}
	return p
}

// resolveProperty 解析属性值，查看其是否具有引用关系
func (app *Application) resolveProperty(properties map[string]interface{}, key string, value interface{}) interface{} {
	if s, o := value.(string); o && strings.HasPrefix(s, "${") {
		refKey := s[2 : len(s)-1]
		if refValue, ok := properties[refKey]; !ok {
			panic(fmt.Errorf("property \"%s\" not config", refKey))
		} else {
			refValue = app.resolveProperty(properties, refKey, refValue)
			properties[key] = refValue
			return refValue
		}
	}
	return value
}

// prepare 准备上下文环境
func (app *Application) prepare() {

	// 配置项加载顺序优先级，从高到低:
	// 1.代码设置
	// 2.命令行参数
	// 3.系统环境变量
	// 4.application-profile.properties
	// 5.application.properties
	// 6.内部默认配置

	// 将通过代码设置的属性值拷贝一份，第 1 层
	apiConfig := NewDefaultProperties()
	for k, v := range app.GetProperties() {
		apiConfig.SetProperty(k, v)
	}

	// 加载默认的应用配置文件，如 application.properties，第 5 层
	appConfig := app.loadProfileConfig("")
	p := NewPriorityProperties(apiConfig, appConfig)

	// 加载系统环境变量，第 3 层
	sysEnv := app.loadSystemEnv()
	p.InsertBefore(sysEnv, appConfig)

	// 加载命令行参数，第 2 层
	cmdArgs := app.loadCmdArgs()
	p.InsertBefore(cmdArgs, sysEnv)

	// 加载特定环境的配置文件，如 application-test.properties
	profile := app.GetProfile()
	if profile == "" {
		keys := []string{SpringProfile, SPRING_PROFILE}
		profile = cast.ToString(p.GetProperty(keys...))
	}
	if profile != "" {
		app.SetProfile(profile) // 第 4 层
		profileConfig := app.loadProfileConfig(profile)
		p.InsertBefore(profileConfig, appConfig)
	}

	// 将重组后的属性值写入 ApplicationContext 属性列表
	properties := p.GetProperties()
	for key, value := range properties {
		value = app.resolveProperty(properties, key, value)
		app.SetProperty(key, value)
	}
}

func (app *Application) stopApplication() {
	for _, bean := range app.Events {
		bean.OnStopApplication(app)
	}
}

// ShutDown 停止应用
func (app *Application) ShutDown() {

	defer SpringLogger.Info("application exited")
	SpringLogger.Info("application exiting")

	// OnStopApplication 是否需要有 Timeout 的 Context？
	// 仔细想想没有必要，程序想要优雅退出就得一直等，等到所有工作
	// 做完，用户如果等不急了可以使用 kill -9 进行硬杀，也就是
	// 是否优雅退出取决于用户。这样的话，OnStopApplication 不
	// 依赖 appCtx 的 Context，就只需要考虑 SafeGoroutine
	// 的退出了，而这只需要 Context 一 cancel 也就完事了。

	// 通知 Bean 销毁
	app.Close(app.stopApplication)
}
