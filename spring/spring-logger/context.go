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

package SpringLogger

import (
	"context"
	"fmt"
	"os"
)

// ContextOutput 为 ContextLogger 定制日志输出格式。
type ContextOutput interface {
	// 输出自定义级别的日志，skip 是相对于当前函数的调用深度。
	Output(c *ContextLogger, skip int, level Level, args ...interface{})
	Outputf(c *ContextLogger, skip int, level Level, format string, args ...interface{})
}

// DefaultContextOutput ContextOutput 的默认实现。
type DefaultContextOutput struct{}

func (c *DefaultContextOutput) Output(_ *ContextLogger, skip int, level Level, args ...interface{}) {
	defaultLogger.Output(skip+1, level, args...)
}

func (c *DefaultContextOutput) Outputf(_ *ContextLogger, skip int, level Level, format string, args ...interface{}) {
	defaultLogger.Outputf(skip+1, level, format, args...)
}

var contextOutput ContextOutput = &DefaultContextOutput{}

// RegisterContextOutput 注册全局 ContextOutput 对象
func RegisterContextOutput(output ContextOutput) {
	if output != nil {
		contextOutput = output
	}
}

// ContextLogger 封装了 context.Context 和自定义标签的 StdLogger 对象。
type ContextLogger struct {
	Ctx context.Context
	Tag string
}

// WithContext ContextLogger 的构造函数，自定义标签可以为空。
func WithContext(ctx context.Context, tag ...string) *ContextLogger {
	if len(tag) == 0 {
		return &ContextLogger{Ctx: ctx}
	} else {
		return &ContextLogger{Ctx: ctx, Tag: tag[0]}
	}
}

// WithTag 返回封装了 context.Context 和自定义标签的 StdLogger 对象。
func (c *ContextLogger) WithTag(tag string) StdLogger {
	return &ContextLogger{Ctx: c.Ctx, Tag: tag}
}

// Trace 输出 TRACE 级别的日志。
func (c *ContextLogger) Trace(args ...interface{}) {
	contextOutput.Output(c, 1, TraceLevel, args...)
}

// Tracef 输出 TRACE 级别的日志。
func (c *ContextLogger) Tracef(format string, args ...interface{}) {
	contextOutput.Outputf(c, 1, TraceLevel, format, args...)
}

// Debug 输出 DEBUG 级别的日志。
func (c *ContextLogger) Debug(args ...interface{}) {
	contextOutput.Output(c, 1, DebugLevel, args...)
}

// Debugf 输出 DEBUG 级别的日志。
func (c *ContextLogger) Debugf(format string, args ...interface{}) {
	contextOutput.Outputf(c, 1, DebugLevel, format, args...)
}

// Info 输出 INFO 级别的日志。
func (c *ContextLogger) Info(args ...interface{}) {
	contextOutput.Output(c, 1, InfoLevel, args...)
}

// Infof 输出 INFO 级别的日志。
func (c *ContextLogger) Infof(format string, args ...interface{}) {
	contextOutput.Outputf(c, 1, InfoLevel, format, args...)
}

// Warn 输出 WARN 级别的日志。
func (c *ContextLogger) Warn(args ...interface{}) {
	contextOutput.Output(c, 1, WarnLevel, args...)
}

// Warnf 输出 WARN 级别的日志。
func (c *ContextLogger) Warnf(format string, args ...interface{}) {
	contextOutput.Outputf(c, 1, WarnLevel, format, args...)
}

// Error 输出 ERROR 级别的日志。
func (c *ContextLogger) Error(args ...interface{}) {
	contextOutput.Output(c, 1, ErrorLevel, args...)
}

// Errorf 输出 ERROR 级别的日志。
func (c *ContextLogger) Errorf(format string, args ...interface{}) {
	contextOutput.Outputf(c, 1, ErrorLevel, format, args...)
}

// Panic 输出 PANIC 级别的日志。
func (c *ContextLogger) Panic(args ...interface{}) {
	contextOutput.Output(c, 1, PanicLevel, args...)
}

// Panicf 输出 PANIC 级别的日志。
func (c *ContextLogger) Panicf(format string, args ...interface{}) {
	contextOutput.Outputf(c, 1, PanicLevel, format, args...)
}

// Fatal 输出 FATAL 级别的日志。
func (c *ContextLogger) Fatal(args ...interface{}) {
	contextOutput.Output(c, 1, FatalLevel, args...)
}

// Fatalf 输出 FATAL 级别的日志。
func (c *ContextLogger) Fatalf(format string, args ...interface{}) {
	contextOutput.Outputf(c, 1, FatalLevel, format, args...)
}

// Print 将日志内容输出到控制台。
func (c *ContextLogger) Print(args ...interface{}) {
	fmt.Fprintln(os.Stdout, args...)
}

// Printf 将日志内容输出到控制台。
func (c *ContextLogger) Printf(format string, args ...interface{}) {
	fmt.Fprintf(os.Stdout, format, args...)
}

// Output 自定义日志级别和调用栈深度，skip 是相对于 Output 的调用栈深度。
func (c *ContextLogger) Output(skip int, level Level, args ...interface{}) {
	contextOutput.Output(c, skip+1, level, args...)
}

// Outputf 自定义日志级别和调用栈深度，skip 是相对于 Output 的调用栈深度。
func (c *ContextLogger) Outputf(skip int, level Level, format string, args ...interface{}) {
	contextOutput.Outputf(c, skip+1, level, format, args...)
}
