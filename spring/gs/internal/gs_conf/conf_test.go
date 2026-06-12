/*
 * Copyright 2024 The Go-Spring Authors.
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

package gs_conf

import (
	"os"
	"testing"

	"github.com/go-spring/spring-core/conf"
	"github.com/go-spring/stdlib/testing/assert"
)

func clean() {
	os.Args = nil
	os.Clearenv()
}

func TestAppConfig(t *testing.T) {
	clean()

	t.Run("local dir resolve error", func(t *testing.T) {
		t.Cleanup(clean)
		_ = os.Setenv("GS_SPRING_APP_CONFIG_DIR", "${a}")
		_, err := NewAppConfig().Refresh()
		assert.Error(t, err).Matches(`property \"a\" does not exist`)
	})

	t.Run("config file not exist", func(t *testing.T) {
		t.Cleanup(clean)
		_ = os.Setenv("GS_SPRING_APP_CONFIG_DIR", "./nonexistent")
		p, err := NewAppConfig().Refresh()
		assert.That(t, err).Nil()
		assert.That(t, p).NotNil()
	})

	t.Run("success - load from properties file", func(t *testing.T) {
		t.Cleanup(clean)
		_ = os.Setenv("GS_SPRING_APP_CONFIG_DIR", "./testdata/conf")
		p, err := NewAppConfig().Refresh()
		assert.That(t, err).Nil()

		var config struct {
			SpringAppConfigDir string `value:"${spring.app.config.dir:=./conf}"`
			SpringAppName      string `value:"${spring.app.name}"`
			HttpServerAddr     string `value:"${http.server.addr}"`
		}
		err = conf.Bind(p, &config)
		assert.That(t, err).Nil()
		assert.That(t, config.SpringAppConfigDir).Equal("./testdata/conf")
		assert.That(t, config.SpringAppName).Equal("test")
		assert.That(t, config.HttpServerAddr).Equal("0.0.0.0:8080")
	})

	t.Run("env override config file", func(t *testing.T) {
		t.Cleanup(clean)
		_ = os.Setenv("GS_SPRING_APP_CONFIG_DIR", "./testdata/conf")
		_ = os.Setenv("GS_SPRING_APP_NAME", "env-override-app")
		p, err := NewAppConfig().Refresh()
		assert.That(t, err).Nil()

		var config struct {
			SpringAppName string `value:"${spring.app.name}"`
		}
		err = conf.Bind(p, &config)
		assert.That(t, err).Nil()
		assert.That(t, config.SpringAppName).Equal("env-override-app")
	})

	t.Run("cmd args override all", func(t *testing.T) {
		t.Cleanup(clean)
		os.Args = []string{"test", "-D", "spring.app.name=cmd-override-app"}
		_ = os.Setenv("GS_SPRING_APP_CONFIG_DIR", "./testdata/conf")
		_ = os.Setenv("GS_SPRING_APP_NAME", "env-override-app")
		p, err := NewAppConfig().Refresh()
		assert.That(t, err).Nil()

		var config struct {
			SpringAppName string `value:"${spring.app.name}"`
		}
		err = conf.Bind(p, &config)
		assert.That(t, err).Nil()
		assert.That(t, config.SpringAppName).Equal("cmd-override-app")
	})

	t.Run("sys conf override by config file", func(t *testing.T) {
		t.Cleanup(clean)
		c := NewAppConfig()
		c.Properties.Set("spring.app.name", "sysconf-default")
		_ = os.Setenv("GS_SPRING_APP_CONFIG_DIR", "./testdata/conf")
		p, err := c.Refresh()
		assert.That(t, err).Nil()

		var config struct {
			SpringAppName string `value:"${spring.app.name}"`
		}
		err = conf.Bind(p, &config)
		assert.That(t, err).Nil()
		assert.That(t, config.SpringAppName).Equal("test")
	})

	t.Run("profile config override default config", func(t *testing.T) {
		t.Cleanup(clean)
		tmpDir := t.TempDir()
		appProps := tmpDir + "/app.properties"
		err := os.WriteFile(appProps, []byte("spring.app.name=default\nserver.port=8080"), 0644)
		assert.That(t, err).Nil()
		devProps := tmpDir + "/app-dev.properties"
		err = os.WriteFile(devProps, []byte("spring.app.name=dev-app\nserver.port=9090"), 0644)
		assert.That(t, err).Nil()

		_ = os.Setenv("GS_SPRING_APP_CONFIG_DIR", tmpDir)
		_ = os.Setenv("GS_SPRING_PROFILES_ACTIVE", "dev")
		p, err := NewAppConfig().Refresh()
		assert.That(t, err).Nil()

		var config struct {
			SpringAppName string `value:"${spring.app.name}"`
			ServerPort    string `value:"${server.port}"`
		}
		err = conf.Bind(p, &config)
		assert.That(t, err).Nil()
		assert.That(t, config.SpringAppName).Equal("dev-app")
		assert.That(t, config.ServerPort).Equal("9090")
	})

	t.Run("multiple profiles merge", func(t *testing.T) {
		t.Cleanup(clean)
		tmpDir := t.TempDir()
		appProps := tmpDir + "/app.properties"
		err := os.WriteFile(appProps, []byte("spring.app.name=default\ndb.host=localhost"), 0644)
		assert.That(t, err).Nil()
		devProps := tmpDir + "/app-dev.properties"
		err = os.WriteFile(devProps, []byte("spring.app.name=dev-app\nlog.level=debug"), 0644)
		assert.That(t, err).Nil()
		prodProps := tmpDir + "/app-prod.properties"
		err = os.WriteFile(prodProps, []byte("db.host=prod-db.example.com\ndb.port=5432"), 0644)
		assert.That(t, err).Nil()

		_ = os.Setenv("GS_SPRING_APP_CONFIG_DIR", tmpDir)
		_ = os.Setenv("GS_SPRING_PROFILES_ACTIVE", "dev,prod")
		p, err := NewAppConfig().Refresh()
		assert.That(t, err).Nil()

		var config struct {
			SpringAppName string `value:"${spring.app.name}"`
			DbHost        string `value:"${db.host}"`
			DbPort        string `value:"${db.port}"`
			LogLevel      string `value:"${log.level}"`
		}
		err = conf.Bind(p, &config)
		assert.That(t, err).Nil()
		assert.That(t, config.SpringAppName).Equal("dev-app")
		assert.That(t, config.DbHost).Equal("prod-db.example.com")
		assert.That(t, config.DbPort).Equal("5432")
		assert.That(t, config.LogLevel).Equal("debug")
	})

	t.Run("import config file", func(t *testing.T) {
		t.Cleanup(clean)
		tmpDir := t.TempDir()
		importedProps := tmpDir + "/imported.properties"
		err := os.WriteFile(importedProps, []byte("db.host=imported-host\ndb.port=3306"), 0644)
		assert.That(t, err).Nil()
		appProps := tmpDir + "/app.properties"
		err = os.WriteFile(appProps, []byte("spring.app.name=main-app\nspring.app.imports="+importedProps+"\ndb.user=admin"), 0644)
		assert.That(t, err).Nil()

		_ = os.Setenv("GS_SPRING_APP_CONFIG_DIR", tmpDir)
		p, err := NewAppConfig().Refresh()
		assert.That(t, err).Nil()

		var config struct {
			SpringAppName string `value:"${spring.app.name}"`
			DbHost        string `value:"${db.host}"`
			DbPort        string `value:"${db.port}"`
			DbUser        string `value:"${db.user}"`
		}
		err = conf.Bind(p, &config)
		assert.That(t, err).Nil()
		assert.That(t, config.SpringAppName).Equal("main-app")
		assert.That(t, config.DbHost).Equal("imported-host")
		assert.That(t, config.DbPort).Equal("3306")
		assert.That(t, config.DbUser).Equal("admin")
	})

	t.Run("import file not exist", func(t *testing.T) {
		t.Cleanup(clean)
		tmpDir := t.TempDir()
		appProps := tmpDir + "/app.properties"
		err := os.WriteFile(appProps, []byte("spring.app.name=test\nspring.app.imports=/nonexistent/file.properties"), 0644)
		assert.That(t, err).Nil()

		_ = os.Setenv("GS_SPRING_APP_CONFIG_DIR", tmpDir)
		_, err = NewAppConfig().Refresh()
		assert.Error(t, err).NotNil()
	})
}
