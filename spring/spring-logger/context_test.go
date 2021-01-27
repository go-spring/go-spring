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

package SpringLogger_test

import (
	"context"
	"fmt"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/go-spring/spring-logger"
)

func TestDefaultContextOutput(t *testing.T) {
	SpringLogger.RegisterContextOutput(&SpringLogger.DefaultContextOutput{})
	ctx := context.WithValue(context.TODO(), "trace_id", "0689")

	logger := SpringLogger.WithContext(ctx)
	logger.Trace("level:", "trace")
	logger.Tracef("level:%s", "trace")
	logger.Debug("level:", "debug")
	logger.Debugf("level:%s", "debug")
	logger.Info("level:", "info")
	logger.Infof("level:%s", "info")
	logger.Warn("level:", "warn")
	logger.Warnf("level:%s", "warn")
	logger.Error("level:", "error")
	logger.Errorf("level:%s", "error")

	func() {
		defer func() { fmt.Println(recover()) }()
		logger.Panic("level:", "panic")
	}()

	func() {
		defer func() { fmt.Println(recover()) }()
		logger.Panicf("level:%s", "panic")
	}()

	// logger.Fatal("level:", "fatal")
	// logger.Fatalf("level:%s", "fatal")

	// 上面的代码在控制台上输出如下信息:
	// [INFO] spring-logger/spring-context-logger_test.go:39 level:info
	// [INFO] spring-logger/spring-context-logger_test.go:40 level:info
	// [WARN] spring-logger/spring-context-logger_test.go:41 level:warn
	// [WARN] spring-logger/spring-context-logger_test.go:42 level:warn
	// [ERROR] spring-logger/spring-context-logger_test.go:43 level:error
	// [ERROR] spring-logger/spring-context-logger_test.go:44 level:error
	// [PANIC] spring-logger/spring-context-logger_test.go:48 level:panic
	// [PANIC] spring-logger/spring-context-logger_test.go:53 level:panic

	logger.WithTag("__in").Trace("level:", "trace")
	logger.WithTag("__in").Tracef("level:%s", "trace")
	logger.WithTag("__in").Debug("level:", "debug")
	logger.WithTag("__in").Debugf("level:%s", "debug")
	logger.WithTag("__in").Info("level:", "info")
	logger.WithTag("__in").Infof("level:%s", "info")
	logger.WithTag("__in").Warn("level:", "warn")
	logger.WithTag("__in").Warnf("level:%s", "warn")
	logger.WithTag("__in").Error("level:", "error")
	logger.WithTag("__in").Errorf("level:%s", "error")

	func() {
		defer func() { fmt.Println(recover()) }()
		logger.WithTag("__in").Panic("level:", "panic")
	}()

	func() {
		defer func() { fmt.Println(recover()) }()
		logger.WithTag("__in").Panicf("level:%s", "panic")
	}()

	//logger.WithTag("__in").Fatal("level:", "fatal")
	//logger.WithTag("__in").Fatalf("level:%s", "fatal")

	// 上面的代码在控制台上输出如下信息:
	// [INFO] spring-logger/spring-context-logger_test.go:73 level:info
	// [INFO] spring-logger/spring-context-logger_test.go:74 level:info
	// [WARN] spring-logger/spring-context-logger_test.go:75 level:warn
	// [WARN] spring-logger/spring-context-logger_test.go:76 level:warn
	// [ERROR] spring-logger/spring-context-logger_test.go:77 level:error
	// [ERROR] spring-logger/spring-context-logger_test.go:78 level:error
	// [PANIC] spring-logger/spring-context-logger_test.go:82 level:panic
	// [PANIC] spring-logger/spring-context-logger_test.go:87 level:panic
}

type NativeLogger struct{}

func (l *NativeLogger) Output(c *SpringLogger.ContextLogger, skip int, level SpringLogger.Level, args ...interface{}) {
	l.Outputf(c, skip+1, level, fmt.Sprint(args...))
}

func (l *NativeLogger) Outputf(c *SpringLogger.ContextLogger, skip int, level SpringLogger.Level, format string, args ...interface{}) {

	tag := c.Tag
	if len(tag) > 0 {
		tag += " "
	}

	ctxString := func(ctx context.Context) string {
		if v := ctx.Value("trace_id"); v != nil {
			return "trace_id:" + v.(string)
		}
		return ""
	}

	_, file, line, _ := runtime.Caller(skip + 1)
	_, file = filepath.Split(file)
	str := fmt.Sprintf("[%s] %s:%d %s %s", strings.ToUpper(level.String()), file, line, ctxString(c.Ctx), tag)
	fmt.Println(str + fmt.Sprintf(format, args...))
}

func TestContextLogger(t *testing.T) {
	SpringLogger.RegisterContextOutput(&NativeLogger{})
	ctx := context.WithValue(context.TODO(), "trace_id", "0689")

	logger := SpringLogger.WithContext(ctx)
	logger.Trace("level:", "trace")
	logger.Tracef("level:%s", "trace")
	logger.Debug("level:", "debug")
	logger.Debugf("level:%s", "debug")
	logger.Info("level:", "info")
	logger.Infof("level:%s", "info")
	logger.Warn("level:", "warn")
	logger.Warnf("level:%s", "warn")
	logger.Error("level:", "error")
	logger.Errorf("level:%s", "error")
	logger.Panic("level:", "panic")
	logger.Panicf("level:%s", "panic")
	logger.Fatal("level:", "fatal")
	logger.Fatalf("level:%s", "fatal")

	// 上面的代码在控制台上输出如下信息:
	// [TRACE] spring-context-logger_test.go:135 trace_id:0689 level:trace
	// [TRACE] spring-context-logger_test.go:136 trace_id:0689 level:trace
	// [DEBUG] spring-context-logger_test.go:137 trace_id:0689 level:debug
	// [DEBUG] spring-context-logger_test.go:138 trace_id:0689 level:debug
	// [INFO] spring-context-logger_test.go:139 trace_id:0689 level:info
	// [INFO] spring-context-logger_test.go:140 trace_id:0689 level:info
	// [WARN] spring-context-logger_test.go:141 trace_id:0689 level:warn
	// [WARN] spring-context-logger_test.go:142 trace_id:0689 level:warn
	// [ERROR] spring-context-logger_test.go:143 trace_id:0689 level:error
	// [ERROR] spring-context-logger_test.go:144 trace_id:0689 level:error
	// [PANIC] spring-context-logger_test.go:145 trace_id:0689 level:panic
	// [PANIC] spring-context-logger_test.go:146 trace_id:0689 level:panic
	// [FATAL] spring-context-logger_test.go:147 trace_id:0689 level:fatal
	// [FATAL] spring-context-logger_test.go:148 trace_id:0689 level:fatal

	logger.WithTag("__in").Trace("level:", "trace")
	logger.WithTag("__in").Tracef("level:%s", "trace")
	logger.WithTag("__in").Debug("level:", "debug")
	logger.WithTag("__in").Debugf("level:%s", "debug")
	logger.WithTag("__in").Info("level:", "info")
	logger.WithTag("__in").Infof("level:%s", "info")
	logger.WithTag("__in").Warn("level:", "warn")
	logger.WithTag("__in").Warnf("level:%s", "warn")
	logger.WithTag("__in").Error("level:", "error")
	logger.WithTag("__in").Errorf("level:%s", "error")
	logger.WithTag("__in").Panic("level:", "panic")
	logger.WithTag("__in").Panicf("level:%s", "panic")
	logger.WithTag("__in").Fatal("level:", "fatal")
	logger.WithTag("__in").Fatalf("level:%s", "fatal")

	// 上面的代码在控制台上输出如下信息:
	// [TRACE] spring-context-logger_test.go:166 trace_id:0689 __in level:trace
	// [TRACE] spring-context-logger_test.go:167 trace_id:0689 __in level:trace
	// [DEBUG] spring-context-logger_test.go:168 trace_id:0689 __in level:debug
	// [DEBUG] spring-context-logger_test.go:169 trace_id:0689 __in level:debug
	// [INFO] spring-context-logger_test.go:170 trace_id:0689 __in level:info
	// [INFO] spring-context-logger_test.go:171 trace_id:0689 __in level:info
	// [WARN] spring-context-logger_test.go:172 trace_id:0689 __in level:warn
	// [WARN] spring-context-logger_test.go:173 trace_id:0689 __in level:warn
	// [ERROR] spring-context-logger_test.go:174 trace_id:0689 __in level:error
	// [ERROR] spring-context-logger_test.go:175 trace_id:0689 __in level:error
	// [PANIC] spring-context-logger_test.go:176 trace_id:0689 __in level:panic
	// [PANIC] spring-context-logger_test.go:177 trace_id:0689 __in level:panic
	// [FATAL] spring-context-logger_test.go:178 trace_id:0689 __in level:fatal
	// [FATAL] spring-context-logger_test.go:179 trace_id:0689 __in level:fatal
}
