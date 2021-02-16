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

package boot

import (
	"fmt"
	"os"
	"syscall"
	"testing"
	"time"

	"github.com/go-spring/spring-core/core"
	"github.com/go-spring/spring-core/util"
)

func startApplication(cfgLocation ...string) *application {
	app := NewApp()
	app.Property("application-event.collection", "[]?")
	app.Property("command-line-runner.collection", "[]?")
	app.start(cfgLocation...)
	return app
}

func TestConfig(t *testing.T) {

	t.Run("default config", func(t *testing.T) {
		os.Clearenv()
		app := startApplication()
		util.AssertEqual(t, app.cfgLocation, []string{DefaultConfigLocation})
		util.AssertEqual(t, app.ApplicationContext().GetProfile(), "")
	})

	t.Run("config via env", func(t *testing.T) {
		os.Clearenv()
		_ = os.Setenv(SpringProfile, "dev")
		app := startApplication("testdata/config/")
		util.AssertEqual(t, app.ApplicationContext().GetProfile(), "dev")
	})

	t.Run("config via env 2", func(t *testing.T) {
		os.Clearenv()
		_ = os.Setenv(SPRING_PROFILE, "dev")
		app := startApplication("testdata/config/")
		util.AssertEqual(t, app.ApplicationContext().GetProfile(), "dev")
	})

	t.Run("profile via config", func(t *testing.T) {
		os.Clearenv()
		app := startApplication("testdata/config/")
		util.AssertEqual(t, app.ApplicationContext().GetProfile(), "test")
	})

	t.Run("profile via env&config", func(t *testing.T) {
		os.Clearenv()
		app := startApplication("testdata/config/")
		util.AssertEqual(t, app.ApplicationContext().GetProfile(), "test")
	})

	t.Run("profile via env&config 2", func(t *testing.T) {
		os.Clearenv()
		_ = os.Setenv(SPRING_PROFILE, "dev")
		app := startApplication("testdata/config/")
		util.AssertEqual(t, app.ApplicationContext().GetProfile(), "dev")
	})

	t.Run("default expect system properties", func(t *testing.T) {
		app := startApplication("testdata/config/")
		app.ApplicationContext().Properties().Range(func(k string, v interface{}) { fmt.Println(k, v) })
	})

	t.Run("filter all system properties", func(t *testing.T) {
		// ExpectSysProperties("^$") // 不加载任何系统环境变量
		app := startApplication("testdata/config/")
		app.ApplicationContext().Properties().Range(func(k string, v interface{}) { fmt.Println(k, v) })
	})
}

func TestApp(t *testing.T) {
	go func() {
		time.Sleep(50 * time.Millisecond)
		_ = syscall.Kill(syscall.Getpid(), syscall.SIGTERM)
	}()
	NewApp().
		Profile("dev").
		BannerMode(BannerModeOff).
		Property("spring.application.name", "test").
		Bean(core.ObjBean(new(int)).WithName("int")).
		Configer(core.Config(func(i *int) { fmt.Println(i) }, "")).
		Run()
	t.Log("success")
}
