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
	"flag"
	"fmt"
	"io/ioutil"
	"os"
	"os/signal"
	"path"
	"regexp"
	"strings"
	"syscall"

	"github.com/go-spring/spring-core/conf"
	"github.com/go-spring/spring-core/log"
	"github.com/go-spring/spring-core/util"
	"github.com/spf13/cast"
)

// defaultBanner 默认的 Banner 字符
const defaultBanner = `
 _______  _______         _______  _______  _______ _________ _        _______ 
(  ____ \(  ___  )       (  ____ \(  ____ )(  ____ )\__   __/( (    /|(  ____ \
| (    \/| (   ) |       | (    \/| (    )|| (    )|   ) (   |  \  ( || (    \/
| |      | |   | | _____ | (_____ | (____)|| (____)|   | |   |   \ | || |      
| | ____ | |   | |(_____)(_____  )|  _____)|     __)   | |   | (\ \) || | ____ 
| | \_  )| |   | |             ) || (      | (\ (      | |   | | \   || | \_  )
| (___) || (___) |       /\____) || )      | ) \ \_____) (___| )  \  || (___) |
(_______)(_______)       \_______)|/       |/   \__/\_______/|/    )_)(_______)
`

// version 版本信息
const version = `go-spring@v1.0.5    http://go-spring.com/`

const (
	BannerModeOff     = 0
	BannerModeConsole = 1
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
	Run(ctx Context)
}

// ApplicationEvent 应用运行过程中的事件
type ApplicationEvent interface {
	OnStartApplication(ctx Context) // 应用启动的事件
	OnStopApplication(ctx Context)  // 应用停止的事件
}

// application 应用
type application struct {
	appCtx ApplicationContext // 应用上下文

	cfgLocation         []string // 配置文件目录
	banner              string   // Banner 的内容
	bannerMode          int      // Banner 的显式模式
	expectSysProperties []string // 期望从系统环境变量中获取到的属性，支持正则表达式

	Events  []ApplicationEvent  `autowire:"${application-event.collection:=[]?}"`
	Runners []CommandLineRunner `autowire:"${command-line-runner.collection:=[]?}"`

	exitChan chan struct{}
}

// NewApplication application 的构造函数
func NewApplication() *application {
	return &application{
		appCtx:              New(),
		cfgLocation:         append([]string{}, DefaultConfigLocation),
		bannerMode:          BannerModeConsole,
		expectSysProperties: []string{`.*`},
		exitChan:            make(chan struct{}),
	}
}

// Start 启动应用
func (app *application) start(cfgLocation ...string) {

	if len(cfgLocation) > 0 {
		app.cfgLocation = cfgLocation
	}

	// 打印 Banner 内容
	if app.bannerMode != BannerModeOff {
		app.printBanner()
	}

	// 准备上下文环境
	app.prepare()

	// 注册 Context 接口
	app.appCtx.Object(app.appCtx).Export((*Context)(nil))

	// 依赖注入、属性绑定、初始化
	app.appCtx.Refresh()

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

// printBanner 打印 Banner 到控制台
func printBanner(banner string) {

	// 确保 Banner 前面有空行
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

	// 确保 Banner 后面有空行
	if banner[len(banner)-1] != '\n' {
		fmt.Println()
	}

	var padding []byte
	if n := (maxLength - len(version)) / 2; n > 0 {
		padding = make([]byte, n)
		for i := range padding {
			padding[i] = ' '
		}
	}
	fmt.Println(string(padding) + version + "\n")
}

// loadCmdArgs 加载命令行参数，形如 -name value 的参数才有效。
func (app *application) loadCmdArgs(p conf.Properties) {
	log.Debugf("load cmd args")
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
}

// loadSystemEnv 加载系统环境变量，用户可以自定义有效环境变量的正则匹配
func (app *application) loadSystemEnv(p conf.Properties) {

	var rex []*regexp.Regexp
	for _, v := range app.expectSysProperties {
		if exp, err := regexp.Compile(v); err != nil {
			panic(err)
		} else {
			rex = append(rex, exp)
		}
	}

	log.Debugf("load system env")
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
}

// loadProfileConfig 加载指定环境的配置文件
func (app *application) loadProfileConfig(p conf.Properties, profile string) {

	fileName := "application"
	if profile != "" {
		fileName += "-" + profile
	}

	var (
		schemeName   string
		fileLocation string
	)

	for _, configLocation := range app.cfgLocation {

		if ss := strings.SplitN(configLocation, ":", 2); len(ss) == 1 {
			fileLocation = ss[0]
		} else {
			schemeName = ss[0]
			fileLocation = ss[1]
		}

		scheme, ok := conf.FindScheme(schemeName)
		if !ok {
			panic(fmt.Errorf("unsupported config scheme %s", schemeName))
		}

		err := scheme.Load(p, fileLocation, fileName, []string{"properties", "yaml", "toml"})
		util.Panic(err).When(err != nil)

		// TODO Trace 打印所有的属性。
	}
}

// prepare 准备上下文环境
func (app *application) prepare() {

	// 配置项加载顺序优先级，从高到低:
	// 1.代码设置(api)
	// 2.命令行参数
	// 3.系统环境变量
	// 4.application-profile.conf
	// 5.application.conf
	// 6.内部默认配置

	apiConfig := conf.New()
	cmdConfig := conf.New()
	envConfig := conf.New()
	profileConfig := conf.New()
	defaultConfig := conf.New()

	priority := conf.Priority([]conf.Properties{
		apiConfig, cmdConfig, envConfig,
		profileConfig, defaultConfig,
	})

	// 1. 保存通过代码设置的属性
	if p := app.appCtx.Properties(); p != nil {
		for _, k := range p.Keys() {
			apiConfig.Set(k, p.Get(k))
		}
	}

	// 2. 保存从命令行得到的属性
	app.loadCmdArgs(cmdConfig)

	// 3. 保存从环境变量得到的属性
	app.loadSystemEnv(envConfig)

	// 4. 加载默认的配置文件
	app.loadProfileConfig(defaultConfig, "")

	// 5. 加载 profile 对应的配置文件
	profile := app.appCtx.Profile()
	if profile == "" {
		keys := []string{SpringProfile, SPRING_PROFILE}
		v := priority.GetFirst(keys...)
		profile = cast.ToString(v)
	}
	if profile != "" {
		app.appCtx.SetProfile(profile)
		app.loadProfileConfig(profileConfig, profile)
	}

	// 将重组后的属性值写入 Context 属性列表
	for _, key := range priority.Keys() {
		value, err := conf.Resolve(priority, priority.Get(key))
		util.Panic(err).When(err != nil)
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

// ShutDown 关闭执行器
func (app *application) ShutDown() {
	select {
	case <-app.exitChan:
		// chan 已关闭，无需再次关闭。
	default:
		close(app.exitChan)
	}
}
