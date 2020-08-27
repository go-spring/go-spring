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

// Logger 获取一个标准的 Logger 接口
var Logger func(ctx context.Context, tags ...string) StdLogger

// LoggerContext 带有 Logger 功能的 context.Context 接口
type LoggerContext interface {
	// PrefixLogger 带有前缀的 Logger 接口
	PrefixLogger

	// Context 获取标准 Context 对象
	Context() context.Context

	// Logger 获取标准 Logger 接口
	Logger(tags ...string) StdLogger
}

// DefaultLoggerContext 默认的 LoggerContext 版本
type DefaultLoggerContext struct {
	ctx context.Context
}

// NewDefaultLoggerContext DefaultLoggerContext 的构造函数
func NewDefaultLoggerContext(ctx context.Context) *DefaultLoggerContext {
	return &DefaultLoggerContext{ctx: ctx}
}

// Context 获取标准 Context 对象
func (c *DefaultLoggerContext) Context() context.Context {
	return c.ctx
}

func (c *DefaultLoggerContext) logger(wrapper bool, tags ...string) StdLogger {
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

// Logger 获取标准 Logger 接口
func (c *DefaultLoggerContext) Logger(tags ...string) StdLogger {
	return c.logger(true, tags...)
}

func (c *DefaultLoggerContext) LogTrace(args ...interface{}) {
	c.logger(false).Trace(args...)
}

func (c *DefaultLoggerContext) LogTracef(format string, args ...interface{}) {
	c.logger(false).Tracef(format, args...)
}

func (c *DefaultLoggerContext) LogDebug(args ...interface{}) {
	c.logger(false).Debug(args...)
}

func (c *DefaultLoggerContext) LogDebugf(format string, args ...interface{}) {
	c.logger(false).Debugf(format, args...)
}

func (c *DefaultLoggerContext) LogInfo(args ...interface{}) {
	c.logger(false).Info(args...)
}

func (c *DefaultLoggerContext) LogInfof(format string, args ...interface{}) {
	c.logger(false).Infof(format, args...)
}

func (c *DefaultLoggerContext) LogWarn(args ...interface{}) {
	c.logger(false).Warn(args...)
}

func (c *DefaultLoggerContext) LogWarnf(format string, args ...interface{}) {
	c.logger(false).Warnf(format, args...)
}

func (c *DefaultLoggerContext) LogError(args ...interface{}) {
	c.logger(false).Error(args...)
}

func (c *DefaultLoggerContext) LogErrorf(format string, args ...interface{}) {
	c.logger(false).Errorf(format, args...)
}

func (c *DefaultLoggerContext) LogPanic(args ...interface{}) {
	c.logger(false).Panic(args...)
}

func (c *DefaultLoggerContext) LogPanicf(format string, args ...interface{}) {
	c.logger(false).Panicf(format, args...)
}

func (c *DefaultLoggerContext) LogFatal(args ...interface{}) {
	c.logger(false).Fatal(args...)
}

func (c *DefaultLoggerContext) LogFatalf(format string, args ...interface{}) {
	c.logger(false).Fatalf(format, args...)
}
