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
	"os"
	"strings"

	"github.com/go-spring/go-spring-parent/spring-logger"
	"github.com/go-spring/go-spring/spring-core"
)

const (
	SpringAccess  = "spring.access"  // "all" 为允许注入私有字段
	SpringProfile = "spring.profile" // 运行环境
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
	AppContext     ApplicationContext // 应用上下文
	ConfigLocation []string           // 配置文件目录
}

// newApplication application 的构造函数
func newApplication(appCtx ApplicationContext, cfgLocation []string) *application {
	return &application{
		AppContext:     appCtx,
		ConfigLocation: cfgLocation,
	}
}

// Start 启动 SpringBoot 应用
func (app *application) Start() {

	// 加载系统环境变量
	app.loadSystemEnv()

	// 加载配置文件
	app.loadConfigFiles()

	// 注册 ApplicationContext
	app.AppContext.RegisterBean(app.AppContext).AsInterface((*ApplicationContext)(nil), (*SpringCore.SpringContext)(nil))

	// 依赖注入、属性绑定、Bean 初始化
	app.AppContext.AutoWireBeans()

	// 执行命令行启动器
	var runners []CommandLineRunner
	app.AppContext.CollectBeans(&runners)

	for _, r := range runners {
		r.Run(app.AppContext)
	}

	// 通知应用启动事件
	var eventBeans []ApplicationEvent
	app.AppContext.CollectBeans(&eventBeans)

	for _, bean := range eventBeans {
		bean.OnStartApplication(app.AppContext)
	}

	SpringLogger.Info("spring boot started")
}

func (app *application) loadSystemEnv() {
	SpringLogger.Debugf(">>> load system env")
	for _, env := range os.Environ() {
		if i := strings.Index(env, "="); i > 0 {
			k, v := env[0:i], env[i+1:]
			k = strings.ToLower(k)
			SpringLogger.Tracef("%s=%v", k, v)
			app.AppContext.SetProperty(k, v)
		}
	}
}

func (app *application) loadConfigFiles() {

	// 加载默认的应用配置文件，如 application.properties
	app.loadProfileConfig("")

	// 加载用户设置的配置文件，如 application-test.properties
	if profile := app.AppContext.GetProfile(); profile != "" {
		app.loadProfileConfig(profile)
	}
}

func (app *application) loadProfileConfig(profile string) {
	for _, configLocation := range app.ConfigLocation {

		var result map[string]interface{}

		if ss := strings.Split(configLocation, ":"); len(ss) == 1 {
			result = NewDefaultPropertySource(ss[0]).Load(profile)
		} else {
			switch ss[0] {
			case "k8s":
				result = NewConfigMapPropertySource(ss[1]).Load(profile)
			}
		}

		for k, v := range result {
			app.AppContext.SetProperty(k, v)
		}
	}
}

// ShutDown 停止 SpringBoot 应用
func (app *application) ShutDown() {

	// 通知 Bean 销毁
	app.AppContext.Close()

	// 通知应用停止事件
	var eventBeans []ApplicationEvent
	app.AppContext.CollectBeans(&eventBeans)

	for _, bean := range eventBeans {
		bean.OnStopApplication(app.AppContext)
	}

	// 等待所有 goroutine 退出
	app.AppContext.Wait()

	SpringLogger.Info("spring boot exited")
}
