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
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/go-spring/spring-logger"
)

///////////////////////// 模拟用户已有的日志组件 /////////////////////////

type NativeLogger struct{}

const NativeLoggerCallerSkip = 2

func (_ *NativeLogger) CtxString(ctx context.Context) string {
	if v := ctx.Value("trace_id"); v != nil {
		return "trace_id:" + v.(string)
	}
	return ""
}

// Printf 提供一个无需自定义调用栈深度的函数
func (l *NativeLogger) Printf(ctx context.Context, tag string, level string, format string, args ...interface{}) {
	l.Output(NativeLoggerCallerSkip, ctx, tag, level, format, args...)
}

// Output 提供一个可以自定义调用栈深度的函数
func (l *NativeLogger) Output(callerSkip int, ctx context.Context, tag string, level string, format string, args ...interface{}) {
	if len(tag) > 0 {
		tag += " "
	}
	_, file, line, _ := runtime.Caller(callerSkip)
	_, file = filepath.Split(file)
	str := fmt.Sprintf("%s %s:%d %s %s", level, file, line, l.CtxString(ctx), tag)
	str += fmt.Sprintf(format, args...)
	fmt.Println(str)
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

func (l *ContextLogger) printf(level string, format string, args ...interface{}) {
	var tag string
	if len(l.tags) > 0 {
		tag = l.tags[0]
	}
	nativeLogger.Output(NativeLoggerCallerSkip+2, l.ctx, tag, level, format, args...)
}

func (l *ContextLogger) Tracef(format string, args ...interface{}) {
	l.printf("[TRACE]", format, args...)
}

func (l *ContextLogger) Trace(args ...interface{}) {
	fmt.Println(args...)
}

func (l *ContextLogger) Debugf(format string, args ...interface{}) {
	l.printf("[DEBUG]", format, args...)
}

func (l *ContextLogger) Debug(args ...interface{}) {
	fmt.Println(args...)
}

func (l *ContextLogger) Infof(format string, args ...interface{}) {
	l.printf("[INFO]", format, args...)
}

func (l *ContextLogger) Info(args ...interface{}) {
	fmt.Println(args...)
}

func (l *ContextLogger) Warnf(format string, args ...interface{}) {
	l.printf("[WARN]", format, args...)
}

func (l *ContextLogger) Warn(args ...interface{}) {
	fmt.Println(args...)
}

func (l *ContextLogger) Errorf(format string, args ...interface{}) {
	l.printf("[ERROR]", format, args...)
}

func (l *ContextLogger) Error(args ...interface{}) {
	fmt.Println(args...)
}

func (l *ContextLogger) Panicf(format string, args ...interface{}) {
	str := fmt.Sprintf(format, args...)
	l.printf("[PANIC]", str)
	panic(errors.New(str))
}

func (l *ContextLogger) Panic(args ...interface{}) {
	fmt.Println(args...)
	panic(errors.New(""))
}

func (l *ContextLogger) Fatalf(format string, args ...interface{}) {
	l.printf("[FATAL]", format, args...)
	os.Exit(1)
}

func (l *ContextLogger) Fatal(args ...interface{}) {
	fmt.Println(args...)
	os.Exit(1)
}

func (l *ContextLogger) Printf(format string, args ...interface{}) {
	fmt.Printf(format, args...)
}

func (l *ContextLogger) Print(args ...interface{}) {
	fmt.Println(args...)
}

func TestDefaultTraceContext(t *testing.T) {
	ctx := context.WithValue(context.TODO(), "trace_id", "0689")

	l := &NativeLogger{}
	l.Printf(ctx, "", "[DEBUG]", "level:%s %d", "debug", 0)
	l.Printf(ctx, "", "[INFO]", "level:%s %d", "info", 1)
	l.Printf(ctx, "", "[WARN]", "level:%s %d", "warn", 2)
	l.Printf(ctx, "", "[ERROR]", "level:%s %d", "error", 3)

	// 上面的代码在控制台上输出如下信息:
	// [DEBUG] spring-logger-context_test.go:160 trace_id:0689 level:debug 0
	// [INFO] spring-logger-context_test.go:161 trace_id:0689 level:info 1
	// [WARN] spring-logger-context_test.go:162 trace_id:0689 level:warn 2
	// [ERROR] spring-logger-context_test.go:163 trace_id:0689 level:error 3

	tracer := SpringLogger.NewDefaultLoggerContext(ctx)

	fmt.Println()

	tracer.LogDebugf("level:%s %d", "debug", 0)
	tracer.LogInfof("level:%s %d", "info", 1)
	tracer.LogWarnf("level:%s %d", "warn", 2)
	tracer.LogErrorf("level:%s %d", "error", 3)
	//tracer.LogPanicf("level:%s %d", "panic", 4)
	//tracer.LogFatalf("level:%s %d", "fatal", 5)

	// 上面的代码在控制台上输出如下信息:
	// [DEBUG] spring-logger-context_test.go:175 trace_id:0689 level:debug 0
	// [INFO] spring-logger-context_test.go:176 trace_id:0689 level:info 1
	// [WARN] spring-logger-context_test.go:177 trace_id:0689 level:warn 2
	// [ERROR] spring-logger-context_test.go:178 trace_id:0689 level:error 3

	fmt.Println()

	tracer.Logger("__in").Debugf("level:%s %d", "debug", 0)
	tracer.Logger("__in").Infof("level:%s %d", "info", 1)
	tracer.Logger("__in").Warnf("level:%s %d", "warn", 2)
	tracer.Logger("__in").Errorf("level:%s %d", "error", 3)
	//tracer.Logger("__in").Panicf("level:%s %d", "panic", 4)
	//tracer.Logger("__in").Fatalf("level:%s %d", "fatal", 5)

	// 上面的代码在控制台上输出如下信息:
	// [DEBUG] spring-logger-context_test.go:190 trace_id:0689 __in level:debug 0
	// [INFO] spring-logger-context_test.go:191 trace_id:0689 __in level:info 1
	// [WARN] spring-logger-context_test.go:192 trace_id:0689 __in level:warn 2
	// [ERROR] spring-logger-context_test.go:193 trace_id:0689 __in level:error 3

	fmt.Println()
}
