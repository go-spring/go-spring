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
	"path/filepath"
	"testing"

	"go-spring.org/spring/conf"
	"go-spring.org/stdlib/testing/assert"
)

// pairOf is a small helper that returns the value for the given key in a
// list of parsed pairs, or false when the key is absent.
func pairOf(pairs []envPair, key string) (string, bool) {
	for _, kv := range pairs {
		if kv.key == key {
			return kv.value, true
		}
	}
	return "", false
}

func TestParseDotEnv(t *testing.T) {

	t.Run("comments and blank lines", func(t *testing.T) {
		data := []byte("# a comment\n\nKEY1=val1\n   # indented comment\nKEY2=val2\n")
		pairs, err := parseDotEnv(data)
		assert.That(t, err).Nil()
		assert.That(t, len(pairs)).Equal(2)
		v, ok := pairOf(pairs, "KEY1")
		assert.That(t, ok).True()
		assert.That(t, v).Equal("val1")
		v, ok = pairOf(pairs, "KEY2")
		assert.That(t, ok).True()
		assert.That(t, v).Equal("val2")
	})

	t.Run("export prefix", func(t *testing.T) {
		data := []byte("export KEY1=val1\nexport   KEY2=val2\n")
		pairs, err := parseDotEnv(data)
		assert.That(t, err).Nil()
		assert.That(t, len(pairs)).Equal(2)
		v, ok := pairOf(pairs, "KEY1")
		assert.That(t, ok).True()
		assert.That(t, v).Equal("val1")
		v, ok = pairOf(pairs, "KEY2")
		assert.That(t, ok).True()
		assert.That(t, v).Equal("val2")
	})

	t.Run("unquoted value trimmed", func(t *testing.T) {
		data := []byte("KEY =   hello world   \n")
		pairs, err := parseDotEnv(data)
		assert.That(t, err).Nil()
		v, ok := pairOf(pairs, "KEY")
		assert.That(t, ok).True()
		assert.That(t, v).Equal("hello world")
	})

	t.Run("empty value", func(t *testing.T) {
		data := []byte("KEY=\n")
		pairs, err := parseDotEnv(data)
		assert.That(t, err).Nil()
		v, ok := pairOf(pairs, "KEY")
		assert.That(t, ok).True()
		assert.That(t, v).Equal("")
	})

	t.Run("double quoted with escapes", func(t *testing.T) {
		data := []byte(`KEY="line1\nline2\ttabbed\"quote\\slash"`)
		pairs, err := parseDotEnv(data)
		assert.That(t, err).Nil()
		v, ok := pairOf(pairs, "KEY")
		assert.That(t, ok).True()
		assert.That(t, v).Equal("line1\nline2\ttabbed\"quote\\slash")
	})

	t.Run("double quoted multiline", func(t *testing.T) {
		data := []byte("KEY=\"line1\nline2\nline3\"\n")
		pairs, err := parseDotEnv(data)
		assert.That(t, err).Nil()
		v, ok := pairOf(pairs, "KEY")
		assert.That(t, ok).True()
		assert.That(t, v).Equal("line1\nline2\nline3")
	})

	t.Run("double quoted line continuation", func(t *testing.T) {
		data := []byte("KEY=\"a \\\nb c\"\n")
		pairs, err := parseDotEnv(data)
		assert.That(t, err).Nil()
		v, ok := pairOf(pairs, "KEY")
		assert.That(t, ok).True()
		assert.That(t, v).Equal("a b c")
	})

	t.Run("single quoted is literal", func(t *testing.T) {
		data := []byte(`KEY='no \n expansion $VAR ${a.b}'`)
		pairs, err := parseDotEnv(data)
		assert.That(t, err).Nil()
		v, ok := pairOf(pairs, "KEY")
		assert.That(t, ok).True()
		assert.That(t, v).Equal(`no \n expansion $VAR ${a.b}`)
	})

	t.Run("single quoted multiline", func(t *testing.T) {
		data := []byte("KEY='line1\nline2'\n")
		pairs, err := parseDotEnv(data)
		assert.That(t, err).Nil()
		v, ok := pairOf(pairs, "KEY")
		assert.That(t, ok).True()
		assert.That(t, v).Equal("line1\nline2")
	})

	t.Run("trailing comment after quoted value", func(t *testing.T) {
		data := []byte(`KEY="value" # a comment` + "\n")
		pairs, err := parseDotEnv(data)
		assert.That(t, err).Nil()
		v, ok := pairOf(pairs, "KEY")
		assert.That(t, ok).True()
		assert.That(t, v).Equal("value")
	})

	t.Run("hash in unquoted value is literal", func(t *testing.T) {
		data := []byte("KEY=val#1\n")
		pairs, err := parseDotEnv(data)
		assert.That(t, err).Nil()
		v, ok := pairOf(pairs, "KEY")
		assert.That(t, ok).True()
		assert.That(t, v).Equal("val#1")
	})

	t.Run("expand reference to earlier variable", func(t *testing.T) {
		data := []byte("DB_HOST=localhost\nDB_URL=postgres://$DB_HOST:5432/db\n")
		pairs, err := parseDotEnv(data)
		assert.That(t, err).Nil()
		v, ok := pairOf(pairs, "DB_URL")
		assert.That(t, ok).True()
		assert.That(t, v).Equal("postgres://localhost:5432/db")
	})

	t.Run("expand braced reference", func(t *testing.T) {
		data := []byte("DB_HOST=localhost\nDB_URL=postgres://${DB_HOST}/db\n")
		pairs, err := parseDotEnv(data)
		assert.That(t, err).Nil()
		v, ok := pairOf(pairs, "DB_URL")
		assert.That(t, ok).True()
		assert.That(t, v).Equal("postgres://localhost/db")
	})

	t.Run("expand reference to os environment", func(t *testing.T) {
		_ = os.Setenv("GSDOTENV_OS_VAR", "from-os")
		defer func() { _ = os.Unsetenv("GSDOTENV_OS_VAR") }()
		data := []byte("KEY=prefix-$GSDOTENV_OS_VAR-suffix\n")
		pairs, err := parseDotEnv(data)
		assert.That(t, err).Nil()
		v, ok := pairOf(pairs, "KEY")
		assert.That(t, ok).True()
		assert.That(t, v).Equal("prefix-from-os-suffix")
	})

	t.Run("undefined reference expands to empty", func(t *testing.T) {
		_ = os.Unsetenv("GSDOTENV_UNDEFINED")
		data := []byte("KEY=[$GSDOTENV_UNDEFINED]\n")
		pairs, err := parseDotEnv(data)
		assert.That(t, err).Nil()
		v, ok := pairOf(pairs, "KEY")
		assert.That(t, ok).True()
		assert.That(t, v).Equal("[]")
	})

	t.Run("dotted reference left for Go-Spring", func(t *testing.T) {
		data := []byte("KEY=${spring.app.name}\n")
		pairs, err := parseDotEnv(data)
		assert.That(t, err).Nil()
		v, ok := pairOf(pairs, "KEY")
		assert.That(t, ok).True()
		assert.That(t, v).Equal("${spring.app.name}")
	})

	t.Run("escaped dollar is literal", func(t *testing.T) {
		data := []byte(`KEY="cost is \$5"`)
		pairs, err := parseDotEnv(data)
		assert.That(t, err).Nil()
		v, ok := pairOf(pairs, "KEY")
		assert.That(t, ok).True()
		assert.That(t, v).Equal("cost is $5")
	})

	t.Run("missing equals sign", func(t *testing.T) {
		_, err := parseDotEnv([]byte("KEY_WITHOUT_EQ\n"))
		assert.Error(t, err).Matches("missing '=' after key")
	})

	t.Run("invalid variable name", func(t *testing.T) {
		_, err := parseDotEnv([]byte("db.host=localhost\n"))
		assert.Error(t, err).Matches("invalid variable name")
	})

	t.Run("unterminated double quote", func(t *testing.T) {
		_, err := parseDotEnv([]byte(`KEY="unterminated`))
		assert.Error(t, err).Matches("unterminated double-quoted value")
	})

	t.Run("unterminated single quote", func(t *testing.T) {
		_, err := parseDotEnv([]byte("KEY='unterminated"))
		assert.Error(t, err).Matches("unterminated single-quoted value")
	})

	t.Run("unexpected content after quoted value", func(t *testing.T) {
		_, err := parseDotEnv([]byte(`KEY="value" extra`))
		assert.Error(t, err).Matches("unexpected content after quoted value")
	})

	t.Run("last line without trailing newline", func(t *testing.T) {
		data := []byte("KEY1=val1\nKEY2=val2")
		pairs, err := parseDotEnv(data)
		assert.That(t, err).Nil()
		assert.That(t, len(pairs)).Equal(2)
		v, ok := pairOf(pairs, "KEY2")
		assert.That(t, ok).True()
		assert.That(t, v).Equal("val2")
	})

	t.Run("empty value at end of file", func(t *testing.T) {
		data := []byte("KEY=")
		pairs, err := parseDotEnv(data)
		assert.That(t, err).Nil()
		v, ok := pairOf(pairs, "KEY")
		assert.That(t, ok).True()
		assert.That(t, v).Equal("")
	})

	t.Run("carriage return escape and unknown escape", func(t *testing.T) {
		data := []byte(`KEY="a\rb\qc"`)
		pairs, err := parseDotEnv(data)
		assert.That(t, err).Nil()
		v, ok := pairOf(pairs, "KEY")
		assert.That(t, ok).True()
		assert.That(t, v).Equal("a\rb\\qc")
	})

	t.Run("dollar not followed by a name is literal", func(t *testing.T) {
		data := []byte(`KEY="cost $5 each"`)
		pairs, err := parseDotEnv(data)
		assert.That(t, err).Nil()
		v, ok := pairOf(pairs, "KEY")
		assert.That(t, ok).True()
		assert.That(t, v).Equal("cost $5 each")
	})

	t.Run("backslash at end of file is unterminated", func(t *testing.T) {
		_, err := parseDotEnv([]byte(`KEY="val\`))
		assert.Error(t, err).Matches("unterminated double-quoted value")
	})

	t.Run("dollar at end of file is unterminated", func(t *testing.T) {
		_, err := parseDotEnv([]byte("KEY=\"$"))
		assert.Error(t, err).Matches("unterminated double-quoted value")
	})

	t.Run("empty variable name", func(t *testing.T) {
		_, err := parseDotEnv([]byte("=value\n"))
		assert.Error(t, err).Matches("empty variable name")
	})

	t.Run("leading UTF-8 BOM is tolerated", func(t *testing.T) {
		data := []byte("\xEF\xBB\xBFKEY=val")
		pairs, err := parseDotEnv(data)
		assert.That(t, err).Nil()
		v, ok := pairOf(pairs, "KEY")
		assert.That(t, ok).True()
		assert.That(t, v).Equal("val")
	})

	t.Run("CRLF line endings", func(t *testing.T) {
		data := []byte("KEY1=val1\r\nKEY2=\"val2\"\r\nKEY3='val3'\r\n")
		pairs, err := parseDotEnv(data)
		assert.That(t, err).Nil()
		assert.That(t, len(pairs)).Equal(3)
		v, ok := pairOf(pairs, "KEY1")
		assert.That(t, ok).True()
		assert.That(t, v).Equal("val1")
		v, ok = pairOf(pairs, "KEY2")
		assert.That(t, ok).True()
		assert.That(t, v).Equal("val2")
		v, ok = pairOf(pairs, "KEY3")
		assert.That(t, ok).True()
		assert.That(t, v).Equal("val3")
	})

	t.Run("equals sign in value", func(t *testing.T) {
		data := []byte("KEY=a=b=c\n")
		pairs, err := parseDotEnv(data)
		assert.That(t, err).Nil()
		v, ok := pairOf(pairs, "KEY")
		assert.That(t, ok).True()
		assert.That(t, v).Equal("a=b=c")
	})

	t.Run("duplicate keys resolve last-wins in expansion", func(t *testing.T) {
		data := []byte("KEY=first\nKEY=second\nREF=$KEY\n")
		pairs, err := parseDotEnv(data)
		assert.That(t, err).Nil()
		v, ok := pairOf(pairs, "REF")
		assert.That(t, ok).True()
		assert.That(t, v).Equal("second")
	})
}

func TestLoadDotEnv(t *testing.T) {

	t.Run("missing file is skipped", func(t *testing.T) {
		_ = os.Unsetenv(EnvFile)
		// default ".env" in the package directory does not exist
		err := loadDotEnv()
		assert.That(t, err).Nil()
	})

	t.Run("missing configured file is skipped", func(t *testing.T) {
		_ = os.Setenv(EnvFile, filepath.Join(t.TempDir(), "does-not-exist.env"))
		defer func() { _ = os.Unsetenv(EnvFile) }()
		err := loadDotEnv()
		assert.That(t, err).Nil()
	})

	t.Run("loads variables into environment", func(t *testing.T) {
		dir := t.TempDir()
		path := filepath.Join(dir, ".env")
		err := os.WriteFile(path, []byte("GSDOTENV_LOAD_A=alpha\nGSDOTENV_LOAD_B=beta\n"), 0644)
		assert.That(t, err).Nil()
		_ = os.Setenv(EnvFile, path)
		defer func() {
			_ = os.Unsetenv(EnvFile)
			_ = os.Unsetenv("GSDOTENV_LOAD_A")
			_ = os.Unsetenv("GSDOTENV_LOAD_B")
		}()

		err = loadDotEnv()
		assert.That(t, err).Nil()
		assert.That(t, os.Getenv("GSDOTENV_LOAD_A")).Equal("alpha")
		assert.That(t, os.Getenv("GSDOTENV_LOAD_B")).Equal("beta")
	})

	t.Run("does not override existing environment", func(t *testing.T) {
		dir := t.TempDir()
		path := filepath.Join(dir, ".env")
		err := os.WriteFile(path, []byte("GSDOTENV_OVERRIDE=from-file\n"), 0644)
		assert.That(t, err).Nil()
		_ = os.Setenv("GSDOTENV_OVERRIDE", "from-os")
		_ = os.Setenv(EnvFile, path)
		defer func() {
			_ = os.Unsetenv(EnvFile)
			_ = os.Unsetenv("GSDOTENV_OVERRIDE")
		}()

		err = loadDotEnv()
		assert.That(t, err).Nil()
		assert.That(t, os.Getenv("GSDOTENV_OVERRIDE")).Equal("from-os")
	})

	t.Run("parse error is returned", func(t *testing.T) {
		dir := t.TempDir()
		path := filepath.Join(dir, ".env")
		err := os.WriteFile(path, []byte("db.host=bogus\n"), 0644)
		assert.That(t, err).Nil()
		_ = os.Setenv(EnvFile, path)
		defer func() { _ = os.Unsetenv(EnvFile) }()

		err = loadDotEnv()
		assert.Error(t, err).NotNil()
	})

	t.Run("duplicate keys apply last value", func(t *testing.T) {
		dir := t.TempDir()
		path := filepath.Join(dir, ".env")
		err := os.WriteFile(path, []byte("GSDOTENV_DUP=first\nGSDOTENV_DUP=second\n"), 0644)
		assert.That(t, err).Nil()
		_ = os.Unsetenv("GSDOTENV_DUP")
		_ = os.Setenv(EnvFile, path)
		defer func() {
			_ = os.Unsetenv(EnvFile)
			_ = os.Unsetenv("GSDOTENV_DUP")
		}()

		err = loadDotEnv()
		assert.That(t, err).Nil()
		assert.That(t, os.Getenv("GSDOTENV_DUP")).Equal("second")
	})

	t.Run("unreadable file is an error", func(t *testing.T) {
		// A directory is not a readable file and is not "not exist".
		_ = os.Setenv(EnvFile, t.TempDir())
		defer func() { _ = os.Unsetenv(EnvFile) }()
		err := loadDotEnv()
		assert.Error(t, err).NotNil()
	})
}

func TestAppConfigDotEnv(t *testing.T) {

	t.Run("env file feeds config via GS_ prefix", func(t *testing.T) {
		clean()
		t.Cleanup(clean)

		confDir := t.TempDir()
		err := os.WriteFile(filepath.Join(confDir, "app.properties"),
			[]byte("spring.app.name=from-file\nhttp.server.addr=0.0.0.0:8080"), 0644)
		assert.That(t, err).Nil()

		envPath := filepath.Join(t.TempDir(), ".env")
		err = os.WriteFile(envPath, []byte("GS_SPRING_APP_NAME=from-env\n"), 0644)
		assert.That(t, err).Nil()

		_ = os.Setenv("GS_SPRING_APP_CONFIG_DIR", confDir)
		_ = os.Setenv(EnvFile, envPath)
		defer func() {
			_ = os.Unsetenv(EnvFile)
		}()

		p, err := NewAppConfig().Refresh()
		assert.That(t, err).Nil()

		var config struct {
			SpringAppName  string `value:"${spring.app.name}"`
			HttpServerAddr string `value:"${http.server.addr}"`
		}
		err = conf.Bind(p, &config)
		assert.That(t, err).Nil()
		assert.That(t, config.SpringAppName).Equal("from-env")
		assert.That(t, config.HttpServerAddr).Equal("0.0.0.0:8080")
	})

	t.Run("plain env var kept as-is", func(t *testing.T) {
		clean()
		t.Cleanup(clean)

		confDir := t.TempDir()
		err := os.WriteFile(filepath.Join(confDir, "app.properties"),
			[]byte("spring.app.name=from-file\n"), 0644)
		assert.That(t, err).Nil()

		envPath := filepath.Join(t.TempDir(), ".env")
		err = os.WriteFile(envPath, []byte("API_KEY=top-secret\n"), 0644)
		assert.That(t, err).Nil()

		_ = os.Setenv("GS_SPRING_APP_CONFIG_DIR", confDir)
		_ = os.Setenv(EnvFile, envPath)
		defer func() {
			_ = os.Unsetenv(EnvFile)
		}()

		p, err := NewAppConfig().Refresh()
		assert.That(t, err).Nil()

		var config struct {
			APIKey        string `value:"${API_KEY}"`
			SpringAppName string `value:"${spring.app.name}"`
		}
		err = conf.Bind(p, &config)
		assert.That(t, err).Nil()
		assert.That(t, config.APIKey).Equal("top-secret")
		assert.That(t, config.SpringAppName).Equal("from-file")
	})
}
