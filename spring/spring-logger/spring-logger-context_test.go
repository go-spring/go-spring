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

///////////////////////// 模拟用户已有的日志组件 /////////////////////////

type NativeLogger struct{}

func (_ *NativeLogger) CtxString(ctx context.Context) string {
	if v := ctx.Value("trace_id"); v != nil {
		return "trace_id:" + v.(string)
	}
	return ""
}

// Printf 提供一个无需自定义调用栈深度的函数
func (l *NativeLogger) Printf(ctx context.Context, tag string, level string, format string, args ...interface{}) {
	l.Outputf(1, ctx, tag, level, format, args...)
}

// Outputf 提供一个可以自定义调用栈深度的函数，skip 是相对于 Outputf 的调用栈深度。
func (l *NativeLogger) Outputf(callerSkip int, ctx context.Context, tag string, level string, format string, args ...interface{}) {
	if len(tag) > 0 {
		tag += " "
	}
	_, file, line, _ := runtime.Caller(callerSkip + 1)
	_, file = filepath.Split(file)
	str := fmt.Sprintf("%s %s:%d %s %s", level, file, line, l.CtxString(ctx), tag)
	str += fmt.Sprintf(format, args...)
	fmt.Println(str)
}

func TestNativeLogger(t *testing.T) {
	ctx := context.WithValue(context.TODO(), "trace_id", "0689")

	l := &NativeLogger{}
	l.Printf(ctx, " _undef", "[DEBUG]", "level:%s", "debug")
	l.Outputf(0, ctx, " _undef", "[DEBUG]", "level:%s", "debug")

	// 上面的代码在控制台上输出如下信息:
	// [DEBUG] spring-logger-context_test.go:62 trace_id:0689  _undef level:debug
	// [DEBUG] spring-logger-context_test.go:63 trace_id:0689  _undef level:debug
}

/////////////////////// 模拟用户需要封装的日志组件 ///////////////////////

func init() {
	// 设置全局转换函数
	SpringLogger.Logger = func(ctx context.Context, tags ...string) SpringLogger.StdLogger {
		return &ContextLogger{ctx: ctx, tags: tags}
	}
}

// nativeLogger 一般是一个全局日志变量
var nativeLogger = &NativeLogger{}

// ContextLogger 用户需要封装的日志组件
type ContextLogger struct {
	ctx  context.Context
	tags []string
}

// printf 提供一个可以自定义调用栈深度的函数，skip 是相对于 printf 的调用栈深度。
func (l *ContextLogger) printf(skip int, level string, format string, args ...interface{}) {
	var tag string
	if len(l.tags) > 0 {
		tag = l.tags[0]
	}
	nativeLogger.Outputf(skip+1, l.ctx, tag, level, format, args...)
}

func (l *ContextLogger) SetLevel(level SpringLogger.Level) {}

func (l *ContextLogger) Tracef(format string, args ...interface{}) {
	l.printf(1, "[TRACE]", format, args...)
}

func (l *ContextLogger) Trace(args ...interface{}) {
	fmt.Println(args...)
}

func (l *ContextLogger) Debugf(format string, args ...interface{}) {
	l.printf(1, "[DEBUG]", format, args...)
}

func (l *ContextLogger) Debug(args ...interface{}) {
	fmt.Println(args...)
}

func (l *ContextLogger) Infof(format string, args ...interface{}) {
	l.printf(1, "[INFO]", format, args...)
}

func (l *ContextLogger) Info(args ...interface{}) {
	fmt.Println(args...)
}

func (l *ContextLogger) Warnf(format string, args ...interface{}) {
	l.printf(1, "[WARN]", format, args...)
}

func (l *ContextLogger) Warn(args ...interface{}) {
	fmt.Println(args...)
}

func (l *ContextLogger) Errorf(format string, args ...interface{}) {
	l.printf(1, "[ERROR]", format, args...)
}

func (l *ContextLogger) Error(args ...interface{}) {
	fmt.Println(args...)
}

func (l *ContextLogger) Panicf(format string, args ...interface{}) {
	str := fmt.Sprintf(format, args...)
	l.printf(1, "[PANIC]", str)
}

func (l *ContextLogger) Panic(args ...interface{}) {
	str := fmt.Sprint(args...)
	fmt.Println(str)
}

func (l *ContextLogger) Fatalf(format string, args ...interface{}) {
	l.printf(1, "[FATAL]", format, args...)
}

func (l *ContextLogger) Fatal(args ...interface{}) {
	fmt.Println(args...)
}

func (l *ContextLogger) Printf(format string, args ...interface{}) {
	fmt.Printf(format, args...)
}

func (l *ContextLogger) Print(args ...interface{}) {
	fmt.Println(args...)
}

func (l *ContextLogger) Outputf(skip int, level SpringLogger.Level, format string, args ...interface{}) {
	levelString := strings.ToUpper(SpringLogger.LevelToString(level))
	l.printf(skip+1, fmt.Sprintf("[%s]", levelString), format, args...)
}

func (l *ContextLogger) Output(skip int, level SpringLogger.Level, args ...interface{}) {
	fmt.Println(args...)
}

func TestContextLogger(t *testing.T) {
	ctx := context.WithValue(context.TODO(), "trace_id", "0689")

	ctxLogger := &ContextLogger{ctx: ctx}
	ctxLogger.Trace("level:", "trace")
	ctxLogger.Tracef("level:%s", "trace")
	ctxLogger.Debug("level:", "debug")
	ctxLogger.Debugf("level:%s", "debug")
	ctxLogger.Info("level:", "info")
	ctxLogger.Infof("level:%s", "info")
	ctxLogger.Warn("level:", "warn")
	ctxLogger.Warnf("level:%s", "warn")
	ctxLogger.Error("level:", "error")
	ctxLogger.Errorf("level:%s", "error")
	ctxLogger.Panic("level:", "panic")
	ctxLogger.Panicf("level:%s", "panic")
	ctxLogger.Fatal("level:", "fatal")
	ctxLogger.Fatalf("level:%s", "fatal")
	ctxLogger.Output(0, SpringLogger.InfoLevel, "level:", "info")
	ctxLogger.Outputf(0, SpringLogger.InfoLevel, "level:%s", "info")

	// 上面的代码在控制台上输出如下信息:
	// level: trace
	// [TRACE] spring-logger-context_test.go:179 trace_id:0689 level:trace
	// level: debug
	// [DEBUG] spring-logger-context_test.go:181 trace_id:0689 level:debug
	// level: info
	// [INFO] spring-logger-context_test.go:183 trace_id:0689 level:info
	// level: warn
	// [WARN] spring-logger-context_test.go:185 trace_id:0689 level:warn
	// level: error
	// [ERROR] spring-logger-context_test.go:187 trace_id:0689 level:error
	// level:panic
	// [PANIC] spring-logger-context_test.go:189 trace_id:0689 level:panic
	// level: fatal
	// [FATAL] spring-logger-context_test.go:191 trace_id:0689 level:fatal
	// level: info
	// [INFO] spring-logger-context_test.go:193 trace_id:0689 level:info
}

func TestDefaultTraceContext(t *testing.T) {
	ctx := context.WithValue(context.TODO(), "trace_id", "0689")

	tracer := SpringLogger.NewDefaultLoggerContext(ctx)
	tracer.LogTrace("level:", "trace")
	tracer.LogTracef("level:%s", "trace")
	tracer.LogDebug("level:", "debug")
	tracer.LogDebugf("level:%s", "debug")
	tracer.LogInfo("level:", "info")
	tracer.LogInfof("level:%s", "info")
	tracer.LogWarn("level:", "warn")
	tracer.LogWarnf("level:%s", "warn")
	tracer.LogError("level:", "error")
	tracer.LogErrorf("level:%s", "error")
	tracer.LogPanic("level:", "panic")
	tracer.LogPanicf("level:%s", "panic")
	tracer.LogFatal("level:", "fatal")
	tracer.LogFatalf("level:%s", "fatal")

	// 上面的代码在控制台上输出如下信息:
	// level: trace
	// [TRACE] spring-logger-context_test.go:219 trace_id:0689 level:trace
	// level: debug
	// [DEBUG] spring-logger-context_test.go:221 trace_id:0689 level:debug
	// level: info
	// [INFO] spring-logger-context_test.go:223 trace_id:0689 level:info
	// level: warn
	// [WARN] spring-logger-context_test.go:225 trace_id:0689 level:warn
	// level: error
	// [ERROR] spring-logger-context_test.go:227 trace_id:0689 level:error
	// level: panic
	// [PANIC] spring-logger-context_test.go:229 trace_id:0689 level:panic
	// level: fatal
	// [FATAL] spring-logger-context_test.go:231 trace_id:0689 level:fatal

	fmt.Println()

	tracer.Logger("__in").Trace("level:", "trace")
	tracer.Logger("__in").Tracef("level:%s", "trace")
	tracer.Logger("__in").Debug("level:", "debug")
	tracer.Logger("__in").Debugf("level:%s", "debug")
	tracer.Logger("__in").Info("level:", "info")
	tracer.Logger("__in").Infof("level:%s", "info")
	tracer.Logger("__in").Warn("level:", "warn")
	tracer.Logger("__in").Warnf("level:%s", "warn")
	tracer.Logger("__in").Error("level:", "error")
	tracer.Logger("__in").Errorf("level:%s", "error")
	tracer.Logger("__in").Panic("level:", "panic")
	tracer.Logger("__in").Panicf("level:%s", "panic")
	tracer.Logger("__in").Fatal("level:", "fatal")
	tracer.Logger("__in").Fatalf("level:%s", "fatal")

	// 上面的代码在控制台上输出如下信息:
	// level: trace
	// [TRACE] spring-logger-context_test.go:252 trace_id:0689 __in level:trace
	// level: debug
	// [DEBUG] spring-logger-context_test.go:254 trace_id:0689 __in level:debug
	// level: info
	// [INFO] spring-logger-context_test.go:256 trace_id:0689 __in level:info
	// level: warn
	// [WARN] spring-logger-context_test.go:258 trace_id:0689 __in level:warn
	// level: error
	// [ERROR] spring-logger-context_test.go:260 trace_id:0689 __in level:error
	// level: panic
	// [PANIC] spring-logger-context_test.go:262 trace_id:0689 __in level:panic
	// level: fatal
	// [FATAL] spring-logger-context_test.go:264 trace_id:0689 __in level:fatal
}
