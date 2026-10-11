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
	"encoding/json"
	"os"
	"strings"
	"testing"
	"time"

	"go-spring.org/stdlib/flatten"
	"go-spring.org/stdlib/testing/assert"
)

// ReadTestConfig returns the sample logging configuration shared by tests in
// both the internal (log) and external (log_test) test packages. It lives in
// this internal test-only file so the two packages can reuse a single copy.
func ReadTestConfig() map[string]string {
	s := `
{
  "bufferCap": "1KB",
  "bufferSize": 1000,
  "appender": {
    "file": {
      "type": "FileAppender",
      "file": "log.txt",
      "layout!": "JSONLayout{}"
    },
    "console!": "ConsoleAppender{layout=TextLayout{}}",
    "sample!": "SampleAppender{layout.type=TextLayout}"
  },
  "logger": {
    "root": {
      "type": "Logger",
      "level": "warn",
      "appenderRef": {
        "ref": "console"
      }
    },
    "myLogger": {
      "type": "AsyncLogger",
      "level": "trace",
      "tag": "_com_request_in,_com_request_*",
      "bufferSize": "${bufferSize}",
      "appenderRef": [
        {
          "ref": "file"
        },
        {
          "ref": "sample"
        }
      ]
    }
  }
}`

	var m map[string]any
	if err := json.Unmarshal([]byte(s), &m); err != nil {
		panic(err)
	}
	return flatten.Flatten(m)
}

func TestLoggers(t *testing.T) {
	defer Destroy()

	// Before the first refresh no loggers are configured.
	assert.Slice(t, Loggers()).Empty()

	// After a successful refresh the configured loggers are reported with
	// their effective minimum level, sorted by name.
	err := RefreshConfig(ReadTestConfig())
	assert.Error(t, err).Nil()
	assert.Slice(t, Loggers()).Equal([]LoggerInfo{
		{Name: "myLogger", Level: "TRACE"},
		{Name: RootLoggerName, Level: "WARN"},
	})
}

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

	Info(context.Background(), TagAppDef, "hello")
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

func TestRefreshConfigErrors(t *testing.T) {
	defer Destroy()

	oldLoggerMap := loggerMap
	loggerMap = map[string]*LoggerWrapper{}
	t.Cleanup(func() {
		loggerMap = oldLoggerMap
	})

	t.Run("appender ref not found", func(t *testing.T) {
		err := RefreshConfig(map[string]string{
			"logger.root.type":            "Logger",
			"logger.root.appenderRef.ref": "file",
			"appender.console.type":       "ConsoleAppender",
		})
		assert.Error(t, err).Matches(`init appender refs for logger root error: appender file not found`)
	})

	t.Run("appender not concurrent-safe", func(t *testing.T) {
		err := RefreshConfig(map[string]string{
			"logger.root.type":            "Logger",
			"logger.root.appenderRef.ref": "rolling",
			"appender.rolling.type":       "RollingFileAppender",
			"appender.rolling.file":       "app.log",
		})
		assert.Error(t, err).Matches(`init appender refs for logger root error: appender rolling is not concurrent-safe`)
	})

	t.Run("invalid wildcard tag", func(t *testing.T) {
		err := RefreshConfig(map[string]string{
			"appender.console.type":           "ConsoleAppender",
			"logger.myLogger.type":            "Logger",
			"logger.myLogger.tag":             "_app*x",
			"logger.myLogger.appenderRef.ref": "console",
		})
		assert.Error(t, err).Matches(`create logger myLogger error: tag '_app\*x' is invalid`)
	})

	t.Run("logger without tag", func(t *testing.T) {
		err := RefreshConfig(map[string]string{
			"appender.console.type":           "ConsoleAppender",
			"logger.myLogger.type":            "Logger",
			"logger.myLogger.tag":             "",
			"logger.myLogger.appenderRef.ref": "console",
		})
		assert.Error(t, err).Matches(`create logger myLogger error: logger must have attribute 'tag'`)
	})

	t.Run("logger start error", func(t *testing.T) {
		err := RefreshConfig(map[string]string{
			"appender.console.type":           "ConsoleAppender",
			"logger.myLogger.type":            "AsyncLogger",
			"logger.myLogger.tag":             "_app_*",
			"logger.myLogger.bufferSize":      "10",
			"logger.myLogger.appenderRef.ref": "console",
		})
		assert.Error(t, err).Matches(`logger myLogger start error: bufferSize 10 is too small`)
	})
}
