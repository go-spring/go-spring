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
)

// Deprecated: 为了和老版本兼容
type LoggerContext = ContextLogger

// Deprecated: 为了和老版本兼容
var NewDefaultLoggerContext = NewDefaultContextLogger

// Logger 返回封装了 context.Context 和自定义标签的 StdLogger 对象
var Logger func(ctx context.Context, tags ...string) StdLogger

// ContextLogger 封装了 context.Context 对象的日志输出接口
type ContextLogger interface {

	// Logger 返回封装了 context.Context 和自定义标签的 StdLogger 对象
	Logger(tags ...string) StdLogger

	// 输出 TRACE 级别的日志
	LogTrace(args ...interface{})
	LogTracef(format string, args ...interface{})

	// 输出 DEBUG 级别的日志
	LogDebug(args ...interface{})
	LogDebugf(format string, args ...interface{})

	// 输出 INFO 级别的日志
	LogInfo(args ...interface{})
	LogInfof(format string, args ...interface{})

	// 输出 WARN 级别的日志
	LogWarn(args ...interface{})
	LogWarnf(format string, args ...interface{})

	// 输出 ERROR 级别的日志
	LogError(args ...interface{})
	LogErrorf(format string, args ...interface{})

	// 输出 PANIC 级别的日志
	LogPanic(args ...interface{})
	LogPanicf(format string, args ...interface{})

	// 输出 FATAL 级别的日志
	LogFatal(args ...interface{})
	LogFatalf(format string, args ...interface{})
}

// DefaultContextLogger 默认的 ContextLogger 实现
type DefaultContextLogger struct {
	ctx context.Context
}

// NewDefaultContextLogger DefaultContextLogger 的构造函数
func NewDefaultContextLogger(ctx context.Context) *DefaultContextLogger {
	return &DefaultContextLogger{ctx: ctx}
}

func (c *DefaultContextLogger) logger(wrapper bool, tags ...string) StdLogger {
	var l StdLogger

	if Logger != nil {
		l = Logger(c.ctx, tags...)
	} else {
		l = defaultLogger
	}

	if wrapper {
		return &StdLoggerWrapper{l}
	}
	return l
}

// Logger 返回封装了 context.Context 和自定义标签的 StdLogger 对象
func (c *DefaultContextLogger) Logger(tags ...string) StdLogger {
	return c.logger(true, tags...)
}

func (c *DefaultContextLogger) LogTrace(args ...interface{}) {
	c.logger(false).Output(1, TraceLevel, args...)
}

func (c *DefaultContextLogger) LogTracef(format string, args ...interface{}) {
	c.logger(false).Outputf(1, TraceLevel, format, args...)
}

func (c *DefaultContextLogger) LogDebug(args ...interface{}) {
	c.logger(false).Output(1, DebugLevel, args...)
}

func (c *DefaultContextLogger) LogDebugf(format string, args ...interface{}) {
	c.logger(false).Outputf(1, DebugLevel, format, args...)
}

func (c *DefaultContextLogger) LogInfo(args ...interface{}) {
	c.logger(false).Output(1, InfoLevel, args...)
}

func (c *DefaultContextLogger) LogInfof(format string, args ...interface{}) {
	c.logger(false).Outputf(1, InfoLevel, format, args...)
}

func (c *DefaultContextLogger) LogWarn(args ...interface{}) {
	c.logger(false).Output(1, WarnLevel, args...)
}

func (c *DefaultContextLogger) LogWarnf(format string, args ...interface{}) {
	c.logger(false).Outputf(1, WarnLevel, format, args...)
}

func (c *DefaultContextLogger) LogError(args ...interface{}) {
	c.logger(false).Output(1, ErrorLevel, args...)
}

func (c *DefaultContextLogger) LogErrorf(format string, args ...interface{}) {
	c.logger(false).Outputf(1, ErrorLevel, format, args...)
}

func (c *DefaultContextLogger) LogPanic(args ...interface{}) {
	c.logger(false).Output(1, PanicLevel, args...)
}

func (c *DefaultContextLogger) LogPanicf(format string, args ...interface{}) {
	c.logger(false).Outputf(1, PanicLevel, format, args...)
}

func (c *DefaultContextLogger) LogFatal(args ...interface{}) {
	c.logger(false).Output(1, FatalLevel, args...)
}

func (c *DefaultContextLogger) LogFatalf(format string, args ...interface{}) {
	c.logger(false).Outputf(1, FatalLevel, format, args...)
}
