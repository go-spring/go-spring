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
	"errors"
	"os"
	"testing"
	"time"

	"github.com/go-spring/spring-base/assert"
	"github.com/go-spring/spring-core/gs"
)

func startApplication(cfgLocation string, fn func(gs.Environment)) *gs.App {

	app := gs.NewApp()
	gs.Setenv("GS_SPRING_BANNER_VISIBLE", "true")
	gs.Setenv("GS_SPRING_CONFIG_LOCATIONS", cfgLocation)

	type PandoraAware struct{}
	app.Provide(func(b gs.Environment) PandoraAware {
		fn(b)
		return PandoraAware{}
	})

	go func() {
		if err := app.Run(); err != nil {
			panic(err)
		}
	}()

	time.Sleep(100 * time.Millisecond)
	return app
}

func TestConfig(t *testing.T) {

	t.Run("config via env", func(t *testing.T) {
		os.Clearenv()
		gs.Setenv("GS_SPRING_PROFILES_ACTIVE", "dev")
		app := startApplication("testdata/config/", func(e gs.Environment) {
			assert.Equal(t, e.GetProperty("spring.profiles.active"), "dev")
		})
		defer app.ShutDown(errors.New("run test end"))
	})

	t.Run("config via env 2", func(t *testing.T) {
		os.Clearenv()
		gs.Setenv("GS_SPRING_PROFILES_ACTIVE", "dev")
		app := startApplication("testdata/config/", func(e gs.Environment) {
			assert.Equal(t, e.GetProperty("spring.profiles.active"), "dev")
		})
		defer app.ShutDown(errors.New("run test end"))
	})

	t.Run("profile via env&config 2", func(t *testing.T) {
		os.Clearenv()
		gs.Setenv("GS_SPRING_PROFILES_ACTIVE", "dev")
		app := startApplication("testdata/config/", func(e gs.Environment) {
			assert.Equal(t, e.GetProperty("spring.profiles.active"), "dev")
			//keys := e.Properties().Keys()
			//sort.Strings(keys)
			//for _, k := range keys {
			//	fmt.Println(k, "=", e.GetProperty(k))
			//}
		})
		defer app.ShutDown(errors.New("run test end"))
	})
}
