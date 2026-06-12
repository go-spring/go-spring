/*
 * Copyright 2025 The Go-Spring Authors.
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

package log

import (
	"bytes"
	"context"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/go-spring/stdlib/testing/assert"
)

func TestParseExprDuplicateExpandedKey(t *testing.T) {
	for range 100 {
		_, err := parseExpr(map[string]string{
			"db!":     `DB { host = "localhost" }`,
			"db.host": "127.0.0.1",
		})
		assert.Error(t, err).Matches("duplicate key 'db.host'")
	}
}

func TestRefreshConfigWithoutRootKeepsDefaultLoggerLayout(t *testing.T) {
	defer Destroy()

	oldLoggerMap := loggerMap
	loggerMap = map[string]*LoggerWrapper{}
	t.Cleanup(func() {
		loggerMap = oldLoggerMap
	})

	logBuf := bytes.NewBuffer(nil)
	Stdout = logBuf
	t.Cleanup(func() {
		Stdout = os.Stdout
		TimeNow = nil
	})

	TimeNow = func(context.Context) time.Time {
		return time.Time{}
	}

	err := RefreshConfig(nil)
	assert.Error(t, err).Nil()

	Info(context.Background(), TagAppDef, Msg("hello"))
	if !strings.Contains(logBuf.String(), "_app_def||msg=hello\n") {
		t.Fatalf("expected default logger output, got %q", logBuf.String())
	}
}

type refreshTypeMismatchPlugin struct{}

func init() {
	RegisterPlugin[refreshTypeMismatchPlugin]("RefreshTypeMismatchPlugin")
}

func TestRefreshConfigPluginTypeMismatchReturnsError(t *testing.T) {
	defer Destroy()

	oldLoggerMap := loggerMap
	loggerMap = map[string]*LoggerWrapper{}
	t.Cleanup(func() {
		loggerMap = oldLoggerMap
	})

	t.Run("appender", func(t *testing.T) {
		err := RefreshConfig(map[string]string{
			"appender.bad.type": "RefreshTypeMismatchPlugin",
		})
		assert.Error(t, err).Matches(`create appender bad error.*plugin RefreshTypeMismatchPlugin does not implement log.Appender`)
	})

	t.Run("logger", func(t *testing.T) {
		err := RefreshConfig(map[string]string{
			"logger.bad.type": "RefreshTypeMismatchPlugin",
			"logger.bad.tag":  "_app_*",
		})
		assert.Error(t, err).Matches(`create logger bad error.*plugin RefreshTypeMismatchPlugin does not implement log.Logger`)
	})
}

func TestRegisterTagAfterRefreshPanics(t *testing.T) {
	defer Destroy()

	oldLoggerMap := loggerMap
	loggerMap = map[string]*LoggerWrapper{}
	t.Cleanup(func() {
		loggerMap = oldLoggerMap
		delete(tagRegistry, "_app_after_refresh")
	})

	err := RefreshConfig(nil)
	assert.Error(t, err).Nil()
	assert.Panic(t, func() {
		RegisterTag("_app_after_refresh")
	}, "log refresh already done")
}

//func TestRefreshFile(t *testing.T) {
//	t.Cleanup(func() {
//		for _, tag := range tagRegistry {
//			tag.logger = defaultLogger
//		}
//	})
//
//	t.Run("file not exist", func(t *testing.T) {
//		defer func() { Destroy() }()
//		err := RefreshFile("testdata/file-not-exist.yaml")
//		assert.Error(t, err).Matches("open testdata/file-not-exist.yaml")
//	})
//
//	t.Run("already refresh", func(t *testing.T) {
//		defer func() { Destroy() }()
//		err := RefreshFile("testdata/log.YAML")
//		assert.Error(t, err).Nil()
//		err = RefreshFile("testdata/log.YAML")
//		assert.Error(t, err).Matches("log refresh already done")
//	})
//}
//
//func TestRefreshConfig(t *testing.T) {
//	t.Cleanup(func() {
//		for _, tag := range tagRegistry {
//			tag.logger = defaultLogger
//		}
//	})
//
//	t.Run("unsupported file", func(t *testing.T) {
//		defer func() { global.init = false }()
//		err := RefreshReader(nil, ".toml")
//		assert.Error(t, err).Matches("RefreshReader error: unsupported file type .toml")
//	})
//
//	t.Run("appenders section not found", func(t *testing.T) {
//		defer func() { global.init = false }()
//		content := `
//			logger.root.type=Logger
//		`
//		err := RefreshReader(strings.NewReader(content), ".properties")
//		assert.Error(t, err).Matches("appenders section not found")
//	})
//
//	t.Run("read appenders error", func(t *testing.T) {
//		defer func() { global.init = false }()
//		content := `
//			logger.root.type=Logger
//			appender=ERROR_PROPERTY
//		`
//		err := RefreshReader(strings.NewReader(content), ".properties")
//		assert.Error(t, err).String("read appenders section error >> property conflict at path appender")
//	})
//
//	t.Run("read loggers error", func(t *testing.T) {
//		defer func() { global.init = false }()
//		content := `
//			logger.root.type=Logger
//			appender.console.type=Console
//			logger=ERROR_PROPERTY
//		`
//		err := RefreshReader(strings.NewReader(content), ".properties")
//		assert.Error(t, err).Matches("RefreshReader error: toStorage error: property conflict at path logger.*")
//	})
//
//	t.Run("plugin not found - appender", func(t *testing.T) {
//		defer func() { global.init = false }()
//		content := `
//			logger.root.type=Logger
//			appender.console.type=NonExistentAppender
//			logger.test.type=AsyncLogger
//		`
//		err := RefreshReader(strings.NewReader(content), ".properties")
//		assert.Error(t, err).Matches("plugin NonExistentAppender not found")
//	})
//
//	t.Run("plugin not found - logger.root", func(t *testing.T) {
//		defer func() { global.init = false }()
//		content := `
//			logger.root.type=NonExistentLogger
//			appender.console.type=Console
//			appender.console.layout.type=TextLayout
//			logger.test.type=AsyncLogger
//		`
//		err := RefreshReader(strings.NewReader(content), ".properties")
//		assert.Error(t, err).Matches("plugin NonExistentLogger not found")
//	})
//
//	t.Run("logger.root no type", func(t *testing.T) {
//		defer func() { global.init = false }()
//		content := `
//			logger.root.level=debug
//			appender.console.type=Console
//			appender.console.layout.type=TextLayout
//			logger.test.type=AsyncLogger
//		`
//		err := RefreshReader(strings.NewReader(content), ".properties")
//		assert.Error(t, err).Matches("attribute 'type' not found")
//	})
//
//	t.Run("init AppenderRefs error - logger.root", func(t *testing.T) {
//		defer func() { global.init = false }()
//		content := `
//			logger.root.type=Logger
//			logger.root.level=debug
//			logger.root.appenderRef.ref=file
//			appender.console.type=Console
//			appender.console.layout.type=TextLayout
//		`
//		err := RefreshReader(strings.NewReader(content), ".properties")
//		assert.Error(t, err).Matches("appender file not found")
//	})
//
//	t.Run("plugin not found - loggers", func(t *testing.T) {
//		defer func() { global.init = false }()
//		content := `
//			logger.root.type=Logger
//			logger.root.level=debug
//			logger.root.appenderRef.ref=console
//			appender.console.type=Console
//			appender.console.layout.type=TextLayout
//			logger.myLogger.type=NonExistentLogger
//		`
//		err := RefreshReader(strings.NewReader(content), ".properties")
//		assert.Error(t, err).Matches("plugin NonExistentLogger not found")
//	})
//
//	t.Run("loggers no type", func(t *testing.T) {
//		defer func() { global.init = false }()
//		content := `
//			logger.root.type=Logger
//			logger.root.level=debug
//			logger.root.appenderRef.ref=console
//			appender.console.type=Console
//			appender.console.layout.type=TextLayout
//			logger.myLogger.level=info
//		`
//		err := RefreshReader(strings.NewReader(content), ".properties")
//		assert.Error(t, err).Matches("attribute 'type' not found")
//	})
//
//	t.Run("plugin not found - loggers", func(t *testing.T) {
//		defer func() { global.init = false }()
//		content := `
//			logger.root.type=Logger
//			logger.root.level=debug
//			logger.root.appenderRef.ref=console
//			appender.console.type=Console
//			appender.console.layout.type=TextLayout
//			logger.myLogger.type=Logger
//			logger.myLogger.level=info
//			logger.myLogger.appenderRef.ref=file
//		`
//		err := RefreshReader(strings.NewReader(content), ".properties")
//		assert.Error(t, err).Matches("appender file not found")
//	})
//
//	t.Run("loggers no tags", func(t *testing.T) {
//		defer func() { global.init = false }()
//		content := `
//			logger.root.type=Logger
//			logger.root.level=debug
//			logger.root.appenderRef.ref=console
//			appender.console.type=Console
//			appender.console.layout.type=TextLayout
//			logger.myLogger.type=Logger
//			logger.myLogger.level=info
//			logger.myLogger.appenderRef.ref=console
//		`
//		err := RefreshReader(strings.NewReader(content), ".properties")
//		assert.Error(t, err).Matches("logger must have attribute 'tags'")
//	})
//
//	t.Run("loggers tags error", func(t *testing.T) {
//		defer func() { global.init = false }()
//		content := `
//			logger.root.type=Logger
//			logger.root.level=debug
//			logger.root.appenderRef.ref=console
//			appender.console.type=Console
//			appender.console.layout.type=TextLayout
//			logger.myLogger.type=Logger
//			logger.myLogger.level=info
//			logger.myLogger.tags=**
//			logger.myLogger.appenderRef.ref=console
//		`
//		err := RefreshReader(strings.NewReader(content), ".properties")
//		assert.Error(t, err).String(`create logger myLogger error >> tag '**' is invalid`)
//	})
//
//	t.Run("logger start error", func(t *testing.T) {
//		defer func() { global.init = false }()
//		content := `
//			logger.root.type=Logger
//			logger.root.level=debug
//			logger.root.appenderRef.ref=console
//			appender.console.type=Console
//			appender.console.layout.type=TextLayout
//			logger.myLogger.type=AsyncLogger
//			logger.myLogger.level=info
//			logger.myLogger.tags=_app_*
//			logger.myLogger.bufferSize=10
//			logger.myLogger.appenderRef.ref=console
//		`
//		err := RefreshReader(strings.NewReader(content), ".properties")
//		assert.Error(t, err).String("logger myLogger start error >> bufferSize is too small")
//	})
//
//	t.Run("logger start error", func(t *testing.T) {
//		defer func() { global.init = false }()
//		content := `
//			bufferCap=1GB
//			logger.root.type=Logger
//			logger.root.level=debug
//			logger.root.appenderRef.ref=console
//			appender.console.type=Console
//			appender.console.layout.type=TextLayout
//			logger.myLogger.type=AsyncLogger
//			logger.myLogger.level=info
//			logger.myLogger.tags=_app_*
//			logger.myLogger.appenderRef.ref=console
//		`
//		err := RefreshReader(strings.NewReader(content), ".properties")
//		assert.Error(t, err).String(`inject property bufferCap error >> invalid bufferCap: "1GB" >> unhandled size name: "GB"`)
//	})
//
//}
