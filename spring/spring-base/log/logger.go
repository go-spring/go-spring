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

package log

import (
	"context"

	"github.com/go-spring/spring-base/atomic"
)

type Logger struct {
	name  string
	entry BaseEntry
	value atomic.Value
}

type LoggerConfig struct {
	Level     Level
	Appenders []Appender
}

func NewLogger(name string, config *LoggerConfig) *Logger {
	l := &Logger{
		name: name,
	}
	l.entry.logger = l
	l.value.Store(config)
	return l
}

func (l *Logger) Name() string {
	return l.name
}

func (l *Logger) config() *LoggerConfig {
	v := l.value.Load()
	return v.(*LoggerConfig)
}

func (l *Logger) SetLevel(level Level) {
	l.value.Store(&LoggerConfig{
		Level:     level,
		Appenders: l.config().Appenders,
	})
}

// WithSkip 创建包含 skip 信息的 Entry 。
func (l *Logger) WithSkip(n int) BaseEntry {
	return l.entry.WithSkip(n)
}

// WithTag 创建包含 tag 信息的 Entry 。
func (l *Logger) WithTag(tag string) BaseEntry {
	return l.entry.WithTag(tag)
}

// WithContext 创建包含 context.Context 对象的 Entry 。
func (l *Logger) WithContext(ctx context.Context) CtxEntry {
	return l.entry.WithContext(ctx)
}

// Trace 输出 TRACE 级别的日志。
func (l *Logger) Trace(args ...interface{}) {
	printf(TraceLevel, &l.entry, "", args)
}

// Tracef 输出 TRACE 级别的日志。
func (l *Logger) Tracef(format string, args ...interface{}) {
	printf(TraceLevel, &l.entry, format, args)
}

// Debug 输出 DEBUG 级别的日志。
func (l *Logger) Debug(args ...interface{}) {
	printf(DebugLevel, &l.entry, "", args)
}

// Debugf 输出 DEBUG 级别的日志。
func (l *Logger) Debugf(format string, args ...interface{}) {
	printf(DebugLevel, &l.entry, format, args)
}

// Info 输出 INFO 级别的日志。
func (l *Logger) Info(args ...interface{}) {
	printf(InfoLevel, &l.entry, "", args)
}

// Infof 输出 INFO 级别的日志。
func (l *Logger) Infof(format string, args ...interface{}) {
	printf(InfoLevel, &l.entry, format, args)
}

// Warn 输出 WARN 级别的日志。
func (l *Logger) Warn(args ...interface{}) {
	printf(WarnLevel, &l.entry, "", args)
}

// Warnf 输出 WARN 级别的日志。
func (l *Logger) Warnf(format string, args ...interface{}) {
	printf(WarnLevel, &l.entry, format, args)
}

// Error 输出 ERROR 级别的日志。
func (l *Logger) Error(args ...interface{}) {
	printf(ErrorLevel, &l.entry, "", args)
}

// Errorf 输出 ERROR 级别的日志。
func (l *Logger) Errorf(format string, args ...interface{}) {
	printf(ErrorLevel, &l.entry, format, args)
}

// Panic 输出 PANIC 级别的日志。
func (l *Logger) Panic(args ...interface{}) {
	printf(PanicLevel, &l.entry, "", args)
}

// Panicf 输出 PANIC 级别的日志。
func (l *Logger) Panicf(format string, args ...interface{}) {
	printf(PanicLevel, &l.entry, format, args)
}

// Fatal 输出 FATAL 级别的日志。
func (l *Logger) Fatal(args ...interface{}) {
	printf(FatalLevel, &l.entry, "", args)
}

// Fatalf 输出 FATAL 级别的日志。
func (l *Logger) Fatalf(format string, args ...interface{}) {
	printf(FatalLevel, &l.entry, format, args)
}
