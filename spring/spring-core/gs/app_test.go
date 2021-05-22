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

package gs_test

import (
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/go-spring/spring-core/assert"
	"github.com/go-spring/spring-core/conf"
	"github.com/go-spring/spring-core/gs"
)

func startApplication(cfgLocation ...string) (*gs.App, gs.Pandora) {
	app := gs.NewApp(gs.OpenPandora())
	app.Property("application-event.collection", "[]?")
	app.Property("command-line-runner.collection", "[]?")

	var p gs.Pandora
	app.Config(func(b gs.Pandora) { p = b })

	go app.Run(cfgLocation...)
	time.Sleep(100 * time.Millisecond)
	return app, p
}

func TestConfig(t *testing.T) {

	t.Run("config via env", func(t *testing.T) {
		os.Clearenv()
		_ = os.Setenv(conf.SpringProfile, "dev")
		app, p := startApplication("testdata/config/")
		defer app.ShutDown()
		assert.Equal(t, p.Prop(conf.SpringProfile), "dev")
	})

	t.Run("config via env 2", func(t *testing.T) {
		os.Clearenv()
		_ = os.Setenv(gs.SPRING_PROFILE, "dev")
		app, p := startApplication("testdata/config/")
		defer app.ShutDown()
		assert.Equal(t, p.Prop(conf.SpringProfile), "dev")
	})

	t.Run("profile via config", func(t *testing.T) {
		os.Clearenv()
		app, p := startApplication("testdata/config/")
		defer app.ShutDown()
		assert.Equal(t, p.Prop(conf.SpringProfile), "test")
	})

	t.Run("profile via env&config", func(t *testing.T) {
		os.Clearenv()
		app, p := startApplication("testdata/config/")
		defer app.ShutDown()
		assert.Equal(t, p.Prop(conf.SpringProfile), "test")
	})

	t.Run("profile via env&config 2", func(t *testing.T) {
		os.Clearenv()
		_ = os.Setenv(gs.SPRING_PROFILE, "dev")
		app, p := startApplication("testdata/config/")
		defer app.ShutDown()
		assert.Equal(t, p.Prop(conf.SpringProfile), "dev")
	})

	t.Run("default expect system properties", func(t *testing.T) {
		app, p := startApplication("testdata/config/")
		defer app.ShutDown()
		for k, v := range p.Prop(conf.RootKey).(map[string]interface{}) {
			fmt.Println(k, v)
		}
	})

	t.Run("filter all system properties", func(t *testing.T) {
		// ExpectSysProperties("^$") // 不加载任何系统环境变量
		app, p := startApplication("testdata/config/")
		defer app.ShutDown()
		for k, v := range p.Prop(conf.RootKey).(map[string]interface{}) {
			fmt.Println(k, v)
		}
	})
}
