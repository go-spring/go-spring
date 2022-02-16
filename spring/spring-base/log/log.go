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

// Package log 重新定义标准日志接口，可以灵活适配各种日志框架。
package log

import (
	"bytes"
	"context"
	"fmt"
	"strings"

	"github.com/go-spring/spring-base/atomic"
	"github.com/go-spring/spring-base/cast"
	"github.com/go-spring/spring-base/chrono"
	"github.com/go-spring/spring-base/color"
	"github.com/go-spring/spring-base/util"
)

// empty 用于创建其他的 Entry 对象。
var empty = BaseEntry{}

// defaultContext 仅用于 log 单元测试。
var defaultContext context.Context

// config 日志模块全局设置。
var config struct {
	output Output
	level  atomic.Uint32
}

func init() {
	config.output = Console
	config.level.Store(uint32(InfoLevel))
}

// Reset 恢复默认的日志输出配置。
func Reset() {
	SetOutput(Console)
	SetLevel(InfoLevel)
}

const (
	TraceLevel = Level(iota)
	DebugLevel
	InfoLevel
	WarnLevel
	ErrorLevel
	PanicLevel
	FatalLevel
)

// Level 日志输出级别。
type Level uint32

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
	}
	return ""
}

// Output 自定义日志的输出格式。
type Output interface {
	Do(level Level, msg *Message)
}

// FuncOutput 函数的形式自定义日志的输出格式。
type FuncOutput func(level Level, msg *Message)

func (fn FuncOutput) Do(level Level, msg *Message) {
	fn(level, msg)
}

// Console 将日志输出到控制台。
var Console = FuncOutput(func(level Level, msg *Message) {
	defer func() { msg.Reuse() }()
	strLevel := strings.ToUpper(level.String())
	if level >= ErrorLevel {
		strLevel = color.Red.Sprint(strLevel)
	} else if level == WarnLevel {
		strLevel = color.Yellow.Sprint(strLevel)
	} else if level == TraceLevel {
		strLevel = color.Green.Sprint(strLevel)
	}
	var buf bytes.Buffer
	for _, a := range msg.Args() {
		buf.WriteString(cast.ToString(a))
	}
	strTime := msg.Time().Format("2006-01-02T15:04:05.000")
	fileLine := util.Contract(fmt.Sprintf("%s:%d", msg.File(), msg.Line()), 48)
	_, _ = fmt.Printf("[%s][%s][%s] %s\n", strLevel, strTime, fileLine, buf.String())
})

// GetLevel 获取日志的输出级别。
func GetLevel() Level {
	return Level(config.level.Load())
}

// SetLevel 设置日志的输出级别。
func SetLevel(level Level) {
	v := uint32(level)
	for {
		o := config.level.Load()
		if config.level.CompareAndSwap(o, v) {
			break
		}
	}
}

// SetOutput 设置日志的输出格式。
func SetOutput(output Output) {
	if output == nil {
		panic("output is nil")
	}
	config.output = output
}

func do(level Level, e Entry, args []interface{}) {
	msg := newMessage()
	msg.args = args
	msg.tag = e.GetTag()
	msg.ctx = e.GetContext()
	msg.errno = e.GetErrNo()
	ctx := msg.ctx
	if ctx == nil {
		ctx = defaultContext
	}
	msg.time = chrono.Now(ctx)
	msg.file, msg.line, _ = Caller(e.GetSkip()+3, true)
	config.output.Do(level, msg)
}

func output(level Level, e Entry, args []interface{}) {
	if GetLevel() > level {
		return
	}
	if len(args) == 1 {
		if fn, ok := args[0].(func() []interface{}); ok {
			args = fn()
		}
	}
	do(level, e, args)
}

func outputf(level Level, e Entry, format string, args []interface{}) {
	if GetLevel() > level {
		return
	}
	if len(args) == 1 {
		if fn, ok := args[0].(func() []interface{}); ok {
			args = fn()
		}
	}
	do(level, e, []interface{}{fmt.Sprintf(format, args...)})
}

// Skip 创建包含 skip 信息的 Entry 。
func Skip(n int) BaseEntry {
	return empty.Skip(n)
}

// Tag 创建包含 tag 信息的 Entry 。
func Tag(tag string) BaseEntry {
	return empty.Tag(tag)
}

// Ctx 创建包含 context.Context 对象的 Entry 。
func Ctx(ctx context.Context) CtxEntry {
	return empty.Ctx(ctx)
}

// T 将可变参数转换成切片形式。
func T(a ...interface{}) []interface{} {
	return a
}

// EnableTrace 是否允许输出 TRACE 级别的日志。
func EnableTrace() bool {
	return GetLevel() <= TraceLevel
}

// EnableDebug 是否允许输出 DEBUG 级别的日志。
func EnableDebug() bool {
	return GetLevel() <= DebugLevel
}

// EnableInfo 是否允许输出 INFO 级别的日志。
func EnableInfo() bool {
	return GetLevel() <= InfoLevel
}

// EnableWarn 是否允许输出 WARN 级别的日志。
func EnableWarn() bool {
	return GetLevel() <= WarnLevel
}

// EnableError 是否允许输出 ERROR 级别的日志。
func EnableError() bool {
	return GetLevel() <= ErrorLevel
}

// EnablePanic 是否允许输出 PANIC 级别的日志。
func EnablePanic() bool {
	return GetLevel() <= PanicLevel
}

// EnableFatal 是否允许输出 FATAL 级别的日志。
func EnableFatal() bool {
	return GetLevel() <= FatalLevel
}

// Trace 输出 TRACE 级别的日志。
func Trace(args ...interface{}) {
	output(TraceLevel, empty, args)
}

// Tracef 输出 TRACE 级别的日志。
func Tracef(format string, args ...interface{}) {
	outputf(TraceLevel, empty, format, args)
}

// Debug 输出 DEBUG 级别的日志。
func Debug(args ...interface{}) {
	output(DebugLevel, empty, args)
}

// Debugf 输出 DEBUG 级别的日志。
func Debugf(format string, args ...interface{}) {
	outputf(DebugLevel, empty, format, args)
}

// Info 输出 INFO 级别的日志。
func Info(args ...interface{}) {
	output(InfoLevel, empty, args)
}

// Infof 输出 INFO 级别的日志。
func Infof(format string, args ...interface{}) {
	outputf(InfoLevel, empty, format, args)
}

// Warn 输出 WARN 级别的日志。
func Warn(args ...interface{}) {
	output(WarnLevel, empty, args)
}

// Warnf 输出 WARN 级别的日志。
func Warnf(format string, args ...interface{}) {
	outputf(WarnLevel, empty, format, args)
}

// Error 输出 ERROR 级别的日志。
func Error(args ...interface{}) {
	output(ErrorLevel, empty, args)
}

// Errorf 输出 ERROR 级别的日志。
func Errorf(format string, args ...interface{}) {
	outputf(ErrorLevel, empty, format, args)
}

// Panic 输出 PANIC 级别的日志。
func Panic(args ...interface{}) {
	output(PanicLevel, empty, args)
}

// Panicf 输出 PANIC 级别的日志。
func Panicf(format string, args ...interface{}) {
	outputf(PanicLevel, empty, format, args)
}

// Fatal 输出 FATAL 级别的日志。
func Fatal(args ...interface{}) {
	output(FatalLevel, empty, args)
}

// Fatalf 输出 FATAL 级别的日志。
func Fatalf(format string, args ...interface{}) {
	outputf(FatalLevel, empty, format, args)
}
