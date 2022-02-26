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

//go:generate mockgen -source=log.go -destination=mock.go -package=log

// Package log 重新定义标准日志接口，可以灵活适配各种日志框架。
package log

import (
	"context"
	"fmt"

	"github.com/go-spring/spring-base/clock"
	"github.com/go-spring/spring-base/util"
)

const (
	NoneLevel  = Level(-1)
	TraceLevel = Level(iota)
	DebugLevel
	InfoLevel
	WarnLevel
	ErrorLevel
	PanicLevel
	FatalLevel
)

var (
	loggers        = make(map[string]Logger)
	emptyEntry     = &BaseEntry{}
	defaultLogger  = Logger(Console)
	defaultContext context.Context
)

// Logger 自定义日志的输出格式。
type Logger interface {
	Level() Level
	Print(msg *Message)
}

// Level 日志输出级别。
type Level int32

func (level Level) String() string {
	switch level {
	case TraceLevel:
		return "trace"
	case DebugLevel:
		return "debug"
	case InfoLevel:
		return "info"
	case WarnLevel:
		return "warn"
	case ErrorLevel:
		return "error"
	case PanicLevel:
		return "panic"
	case FatalLevel:
		return "fatal"
	default:
		return ""
	}
}

// ResetToDefault 重置为默认配置。
func ResetToDefault() {
	util.MustTestMode()
	defaultContext = nil
	defaultLogger = Logger(Console)
	loggers = make(map[string]Logger)
}

func GetLogger(tag string) Logger {
	v, ok := loggers[tag]
	if !ok {
		return defaultLogger
	}
	return v
}

// SetDefaultLogger 为空 tag 或者未知的 tag 设置相应的 Logger 对象。
func SetDefaultLogger(logger Logger) {
	defaultLogger = logger
}

// RegisterLogger 为指定的 tag 设置对应的 Logger 对象。
func RegisterLogger(logger Logger, tags ...string) {
	for _, tag := range tags {
		loggers[tag] = logger
	}
}

// SetDefaultContext 设置默认的 context.Context 对象。
func SetDefaultContext(ctx context.Context) {
	util.MustTestMode()
	defaultContext = ctx
}

// WithSkip 创建包含 skip 信息的 Entry 。
func WithSkip(n int) BaseEntry {
	return emptyEntry.WithSkip(n)
}

// WithTag 创建包含 tag 信息的 Entry 。
func WithTag(tag string) BaseEntry {
	return emptyEntry.WithTag(tag)
}

// WithContext 创建包含 context.Context 对象的 Entry 。
func WithContext(ctx context.Context) CtxEntry {
	return emptyEntry.WithContext(ctx)
}

// Trace 输出 TRACE 级别的日志。
func Trace(args ...interface{}) {
	printf(TraceLevel, emptyEntry, "", args)
}

// Tracef 输出 TRACE 级别的日志。
func Tracef(format string, args ...interface{}) {
	printf(TraceLevel, emptyEntry, format, args)
}

// Debug 输出 DEBUG 级别的日志。
func Debug(args ...interface{}) {
	printf(DebugLevel, emptyEntry, "", args)
}

// Debugf 输出 DEBUG 级别的日志。
func Debugf(format string, args ...interface{}) {
	printf(DebugLevel, emptyEntry, format, args)
}

// Info 输出 INFO 级别的日志。
func Info(args ...interface{}) {
	printf(InfoLevel, emptyEntry, "", args)
}

// Infof 输出 INFO 级别的日志。
func Infof(format string, args ...interface{}) {
	printf(InfoLevel, emptyEntry, format, args)
}

// Warn 输出 WARN 级别的日志。
func Warn(args ...interface{}) {
	printf(WarnLevel, emptyEntry, "", args)
}

// Warnf 输出 WARN 级别的日志。
func Warnf(format string, args ...interface{}) {
	printf(WarnLevel, emptyEntry, format, args)
}

// Error 输出 ERROR 级别的日志。
func Error(args ...interface{}) {
	printf(ErrorLevel, emptyEntry, "", args)
}

// Errorf 输出 ERROR 级别的日志。
func Errorf(format string, args ...interface{}) {
	printf(ErrorLevel, emptyEntry, format, args)
}

// Panic 输出 PANIC 级别的日志。
func Panic(args ...interface{}) {
	printf(PanicLevel, emptyEntry, "", args)
}

// Panicf 输出 PANIC 级别的日志。
func Panicf(format string, args ...interface{}) {
	printf(PanicLevel, emptyEntry, format, args)
}

// Fatal 输出 FATAL 级别的日志。
func Fatal(args ...interface{}) {
	printf(FatalLevel, emptyEntry, "", args)
}

// Fatalf 输出 FATAL 级别的日志。
func Fatalf(format string, args ...interface{}) {
	printf(FatalLevel, emptyEntry, format, args)
}

func printf(level Level, e Entry, format string, args []interface{}) {
	o := GetLogger(e.Tag())
	if o.Level() > level {
		return
	}
	if len(args) == 1 {
		if fn, ok := args[0].(func() []interface{}); ok {
			args = fn()
		}
	}
	if format == "" {
		doPrint(o, level, e, args)
		return
	}
	doPrint(o, level, e, []interface{}{fmt.Sprintf(format, args...)})
}

func doPrint(o Logger, level Level, e Entry, args []interface{}) {
	msg := newMessage()
	msg.Level = level
	msg.Args = args
	msg.Tag = e.Tag()
	msg.Ctx = e.Context()
	msg.Errno = e.Errno()
	ctx := msg.Ctx
	if ctx == nil {
		ctx = defaultContext
	}
	msg.Time = clock.Now(ctx)
	msg.File, msg.Line, _ = Caller(e.Skip()+3, true)
	o.Print(msg)
}
