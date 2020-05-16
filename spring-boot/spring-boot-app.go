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

// 开箱即用的 Go-Spring 程序启动框架。
package SpringBoot

import (
	"flag"
	"os"
	"regexp"
	"strings"

	"github.com/go-spring/go-spring-parent/spring-logger"
	"github.com/go-spring/go-spring/spring-core"
)

const (
	DefaultConfigLocation = "config/" // 默认的配置文件路径

	SpringAccess   = "spring.access" // "all" 为允许注入私有字段
	SPRING_ACCESS  = "SPRING_ACCESS"
	SpringProfile  = "spring.profile" // 运行环境
	SPRING_PROFILE = "SPRING_PROFILE"
)

var (
	_ = flag.String(SpringAccess, "", "")
	_ = flag.String(SpringProfile, "", "")
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

// application SpringBoot 应用
type application struct {
	appCtx      ApplicationContext // 应用上下文
	cfgLocation []string           // 配置文件目录
	configReady func()             // 配置文件已就绪
}

// newApplication application 的构造函数
func newApplication(appCtx ApplicationContext, cfgLocation ...string) *application {
	if len(cfgLocation) == 0 { // 没有的话用默认的配置文件路径
		cfgLocation = append(cfgLocation, DefaultConfigLocation)
	}
	return &application{
		appCtx:      appCtx,
		cfgLocation: cfgLocation,
	}
}

// Start 启动 SpringBoot 应用
func (app *application) Start() {

	// 准备上下文环境
	app.prepare()

	// 注册 ApplicationContext
	app.appCtx.RegisterBean(app.appCtx)

	// 依赖注入、属性绑定、Bean 初始化
	app.appCtx.AutoWireBeans()

	// 执行命令行启动器
	var runners []CommandLineRunner
	app.appCtx.CollectBeans(&runners)

	for _, r := range runners {
		r.Run(app.appCtx)
	}

	// 通知应用启动事件
	var eventBeans []ApplicationEvent
	app.appCtx.CollectBeans(&eventBeans)

	for _, bean := range eventBeans {
		bean.OnStartApplication(app.appCtx)
	}

	SpringLogger.Info("spring boot started")
}

// loadCmdArgs 加载命令行参数
func (_ *application) loadCmdArgs() SpringCore.Properties {
	SpringLogger.Debugf(">>> load cmd args")
	p := SpringCore.NewDefaultProperties()
	for i := 0; i < len(os.Args); i++ {
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

// loadSystemEnv 加载系统环境变量
func (_ *application) loadSystemEnv() SpringCore.Properties {

	var rex []*regexp.Regexp
	for _, v := range expectSysProperties {
		if exp, err := regexp.Compile(v); err != nil {
			panic(err)
		} else {
			rex = append(rex, exp)
		}
	}

	SpringLogger.Debugf(">>> load system env")
	p := SpringCore.NewDefaultProperties()
	for _, env := range os.Environ() {
		if i := strings.Index(env, "="); i > 0 {
			k, v := env[0:i], env[i+1:]
			for _, r := range rex {
				if r.MatchString(k) {
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
func (app *application) loadProfileConfig(profile string) SpringCore.Properties {
	var result map[string]interface{}
	p := SpringCore.NewDefaultProperties()
	for _, configLocation := range app.cfgLocation {
		if ss := strings.Split(configLocation, ":"); len(ss) == 1 {
			result = NewDefaultPropertySource(ss[0]).Load(profile)
		} else {
			switch ss[0] {
			case "k8s":
				result = NewConfigMapPropertySource(ss[1]).Load(profile)
			}
		}
		for k, v := range result {
			SpringLogger.Tracef("%s=%v", k, v)
			p.SetProperty(k, v)
		}
	}
	return p
}

// prepare 准备上下文环境
func (app *application) prepare() {
	// 配置项加载顺序优先级，从高到低:
	// 1.代码设置
	// 2.命令行参数
	// 3.系统环境变量
	// 4.application-profile.properties
	// 5.application.properties
	// 6.内部默认配置

	// 加载默认的应用配置文件，如 application.properties
	appConfig := app.loadProfileConfig("")

	// 加载系统环境变量
	sysEnv := app.loadSystemEnv()
	p := SpringCore.NewPriorityProperties(appConfig)
	for key, value := range sysEnv.GetProperties() {
		p.SetProperty(key, value)
	}

	// 加载命令行参数
	cmdArgs := app.loadCmdArgs()
	p = SpringCore.NewPriorityProperties(p)
	for key, value := range cmdArgs.GetProperties() {
		p.SetProperty(key, value)
	}

	// 加载特定环境的配置文件，如 application-test.properties
	profile := app.appCtx.GetProfile()
	if profile == "" {
		keys := []string{SpringProfile, SPRING_PROFILE}
		profile = p.GetStringProperty(keys...)
	}
	if profile != "" {
		app.appCtx.SetProfile(profile)
		profileConfig := app.loadProfileConfig(profile)
		p.InsertBefore(profileConfig, appConfig)
	}

	// 拷贝用户使用代码设置的属性值
	p = SpringCore.NewPriorityProperties(p)
	for key, value := range app.appCtx.GetProperties() {
		p.SetProperty(key, value)
	}

	// 将重组后的属性值写入 SpringContext 属性列表
	for key, value := range p.GetProperties() {
		app.appCtx.SetProperty(key, value)
	}

	// 设置是否允许注入私有字段
	if ok := app.appCtx.AllAccess(); !ok {
		keys := []string{SpringAccess, SPRING_ACCESS}
		if access := app.appCtx.GetStringProperty(keys...); access != "" {
			app.appCtx.SetAllAccess(strings.ToLower(access) == "all")
		}
	}

	// 配置文件已就绪
	if app.configReady != nil {
		app.configReady()
	}
}

// ShutDown 停止 SpringBoot 应用
func (app *application) ShutDown() {

	// 通知 Bean 销毁
	app.appCtx.Close()

	// 通知应用停止事件
	var eventBeans []ApplicationEvent
	app.appCtx.CollectBeans(&eventBeans)

	for _, bean := range eventBeans {
		bean.OnStopApplication(app.appCtx)
	}

	// 等待所有 goroutine 退出
	app.appCtx.Wait()

	SpringLogger.Info("spring boot exited")
}
