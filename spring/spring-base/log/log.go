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

	"github.com/go-spring/spring-base/chrono"
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
	outputs        = make(map[string]Output)
	emptyEntry     = &BaseEntry{}
	defaultOutput  = Output(Console)
	defaultContext context.Context
)

// Output 自定义日志的输出格式。
type Output interface {
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

func ClearOutputs() {
	defaultOutput = Output(Console)
	outputs = make(map[string]Output)
}

func GetOutput(tag string) Output {
	v, ok := outputs[tag]
	if !ok {
		return defaultOutput
	}
	return v
}

// RegisterDefaultOutput 为空 tag 或者未知的 tag 设置相应的 Output 对象。
func RegisterDefaultOutput(output Output) {
	defaultOutput = output
}

// RegisterOutput 为指定的 tag 设置对应的 Output 对象。
func RegisterOutput(output Output, tags ...string) {
	for _, tag := range tags {
		outputs[tag] = output
	}
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

// T 将可变参数转换成切片形式。
func T(a ...interface{}) []interface{} {
	return a
}

// Trace 输出 TRACE 级别的日志。
func Trace(args ...interface{}) {
	outputf(TraceLevel, emptyEntry, "", args)
}

// Tracef 输出 TRACE 级别的日志。
func Tracef(format string, args ...interface{}) {
	outputf(TraceLevel, emptyEntry, format, args)
}

// Debug 输出 DEBUG 级别的日志。
func Debug(args ...interface{}) {
	outputf(DebugLevel, emptyEntry, "", args)
}

// Debugf 输出 DEBUG 级别的日志。
func Debugf(format string, args ...interface{}) {
	outputf(DebugLevel, emptyEntry, format, args)
}

// Info 输出 INFO 级别的日志。
func Info(args ...interface{}) {
	outputf(InfoLevel, emptyEntry, "", args)
}

// Infof 输出 INFO 级别的日志。
func Infof(format string, args ...interface{}) {
	outputf(InfoLevel, emptyEntry, format, args)
}

// Warn 输出 WARN 级别的日志。
func Warn(args ...interface{}) {
	outputf(WarnLevel, emptyEntry, "", args)
}

// Warnf 输出 WARN 级别的日志。
func Warnf(format string, args ...interface{}) {
	outputf(WarnLevel, emptyEntry, format, args)
}

// Error 输出 ERROR 级别的日志。
func Error(args ...interface{}) {
	outputf(ErrorLevel, emptyEntry, "", args)
}

// Errorf 输出 ERROR 级别的日志。
func Errorf(format string, args ...interface{}) {
	outputf(ErrorLevel, emptyEntry, format, args)
}

// Panic 输出 PANIC 级别的日志。
func Panic(args ...interface{}) {
	outputf(PanicLevel, emptyEntry, "", args)
}

// Panicf 输出 PANIC 级别的日志。
func Panicf(format string, args ...interface{}) {
	outputf(PanicLevel, emptyEntry, format, args)
}

// Fatal 输出 FATAL 级别的日志。
func Fatal(args ...interface{}) {
	outputf(FatalLevel, emptyEntry, "", args)
}

// Fatalf 输出 FATAL 级别的日志。
func Fatalf(format string, args ...interface{}) {
	outputf(FatalLevel, emptyEntry, format, args)
}

func outputf(level Level, e Entry, format string, args []interface{}) {
	o := GetOutput(e.Tag())
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

func doPrint(o Output, level Level, e Entry, args []interface{}) {
	msg := newMessage()
	msg.level = level
	msg.args = args
	msg.tag = e.Tag()
	msg.ctx = e.Context()
	msg.errno = e.Errno()
	ctx := msg.ctx
	if ctx == nil {
		ctx = defaultContext
	}
	msg.time = chrono.Now(ctx)
	msg.file, msg.line, _ = Caller(e.Skip()+3, true)
	o.Print(msg)
}
