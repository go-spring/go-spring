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

	"github.com/go-spring/stdlib/testing/assert"
)

func TestExtractEnvironments(t *testing.T) {
	os.Clearenv()

	t.Run("empty environment", func(t *testing.T) {
		environ := os.Environ()
		if len(environ) > 0 {
			t.Skipf("Skipping test as environment is not empty")
		}
		p, err := extractEnvironments()
		assert.That(t, err).Nil()
		assert.That(t, p).NotNil()
	})

	t.Run("GS_ prefixed variables", func(t *testing.T) {
		_ = os.Setenv("GS_DB_HOST", "localhost")
		_ = os.Setenv("GS_DB_PORT", "5432")
		_ = os.Setenv("GS_APP_NAME", "MyApp")
		defer func() {
			_ = os.Unsetenv("GS_DB_HOST")
			_ = os.Unsetenv("GS_DB_PORT")
			_ = os.Unsetenv("GS_APP_NAME")
		}()

		p, err := extractEnvironments()
		assert.That(t, err).Nil()
		v, ok := p.Get("db.host")
		assert.That(t, ok).True()
		assert.That(t, v).Equal("localhost")
		v, ok = p.Get("db.port")
		assert.That(t, ok).True()
		assert.That(t, v).Equal("5432")
		v, ok = p.Get("app.name")
		assert.That(t, ok).True()
		assert.That(t, v).Equal("MyApp")
	})

	t.Run("non-GS_ variables preserved as-is", func(t *testing.T) {
		_ = os.Setenv("API_KEY", "secret123")
		_ = os.Setenv("PATH", "/usr/bin")
		defer func() {
			_ = os.Unsetenv("API_KEY")
			_ = os.Unsetenv("PATH")
		}()

		p, err := extractEnvironments()
		assert.That(t, err).Nil()
		v, ok := p.Get("API_KEY")
		assert.That(t, ok).True()
		assert.That(t, v).Equal("secret123")
		v, ok = p.Get("PATH")
		assert.That(t, ok).True()
		assert.That(t, v).Equal("/usr/bin")
	})

	t.Run("mixed GS_ and non-GS_ variables", func(t *testing.T) {
		_ = os.Setenv("GS_SERVER_URL", "https://api.example.com")
		_ = os.Setenv("DEBUG", "true")
		_ = os.Setenv("GS_LOG_LEVEL", "INFO")
		defer func() {
			_ = os.Unsetenv("GS_SERVER_URL")
			_ = os.Unsetenv("DEBUG")
			_ = os.Unsetenv("GS_LOG_LEVEL")
		}()

		p, err := extractEnvironments()
		assert.That(t, err).Nil()
		v, ok := p.Get("server.url")
		assert.That(t, ok).True()
		assert.That(t, v).Equal("https://api.example.com")
		v, ok = p.Get("DEBUG")
		assert.That(t, ok).True()
		assert.That(t, v).Equal("true")
		v, ok = p.Get("log.level")
		assert.That(t, ok).True()
		assert.That(t, v).Equal("INFO")
	})

	t.Run("empty value", func(t *testing.T) {
		_ = os.Setenv("GS_EMPTY_VAR", "")
		defer func() {
			_ = os.Unsetenv("GS_EMPTY_VAR")
		}()

		p, err := extractEnvironments()
		assert.That(t, err).Nil()
		v, ok := p.Get("empty.var")
		assert.That(t, ok).True()
		assert.That(t, v).Equal("")
	})

	t.Run("value with equals sign", func(t *testing.T) {
		_ = os.Setenv("GS_FORMULA", "x=y+z")
		defer func() {
			_ = os.Unsetenv("GS_FORMULA")
		}()

		p, err := extractEnvironments()
		assert.That(t, err).Nil()
		v, ok := p.Get("formula")
		assert.That(t, ok).True()
		assert.That(t, v).Equal("x=y+z")
	})

	t.Run("complex nested structure", func(t *testing.T) {
		_ = os.Setenv("GS_DATABASE_PRIMARY_HOST", "db1.example.com")
		_ = os.Setenv("GS_DATABASE_PRIMARY_PORT", "3306")
		_ = os.Setenv("GS_DATABASE_REPLICA_HOST", "db2.example.com")
		defer func() {
			_ = os.Unsetenv("DATABASE_PRIMARY_HOST")
			_ = os.Unsetenv("DATABASE_PRIMARY_PORT")
			_ = os.Unsetenv("DATABASE_REPLICA_HOST")
		}()

		p, err := extractEnvironments()
		assert.That(t, err).Nil()
		v, ok := p.Get("database.primary.host")
		assert.That(t, ok).True()
		assert.That(t, v).Equal("db1.example.com")
		v, ok = p.Get("database.primary.port")
		assert.That(t, ok).True()
		assert.That(t, v).Equal("3306")
		v, ok = p.Get("database.replica.host")
		assert.That(t, ok).True()
		assert.That(t, v).Equal("db2.example.com")
	})
}
