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
	value atomic.Value
	name  string
	entry BaseEntry
}

type LoggerConfig struct {
	Appenders []Appender
	Level     Level
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

// Trace outputs log with level TraceLevel.
func (l *Logger) Trace(args ...interface{}) {
	printf(TraceLevel, &l.entry, "", args)
}

// Tracef outputs log with level TraceLevel.
func (l *Logger) Tracef(format string, args ...interface{}) {
	printf(TraceLevel, &l.entry, format, args)
}

// Debug outputs log with level DebugLevel.
func (l *Logger) Debug(args ...interface{}) {
	printf(DebugLevel, &l.entry, "", args)
}

// Debugf outputs log with level DebugLevel.
func (l *Logger) Debugf(format string, args ...interface{}) {
	printf(DebugLevel, &l.entry, format, args)
}

// Info outputs log with level InfoLevel.
func (l *Logger) Info(args ...interface{}) {
	printf(InfoLevel, &l.entry, "", args)
}

// Infof outputs log with level InfoLevel.
func (l *Logger) Infof(format string, args ...interface{}) {
	printf(InfoLevel, &l.entry, format, args)
}

// Warn outputs log with level WarnLevel.
func (l *Logger) Warn(args ...interface{}) {
	printf(WarnLevel, &l.entry, "", args)
}

// Warnf outputs log with level WarnLevel.
func (l *Logger) Warnf(format string, args ...interface{}) {
	printf(WarnLevel, &l.entry, format, args)
}

// Error outputs log with level ErrorLevel.
func (l *Logger) Error(args ...interface{}) {
	printf(ErrorLevel, &l.entry, "", args)
}

// Errorf outputs log with level ErrorLevel.
func (l *Logger) Errorf(format string, args ...interface{}) {
	printf(ErrorLevel, &l.entry, format, args)
}

// Panic outputs log with level PanicLevel.
func (l *Logger) Panic(args ...interface{}) {
	printf(PanicLevel, &l.entry, "", args)
}

// Panicf outputs log with level PanicLevel.
func (l *Logger) Panicf(format string, args ...interface{}) {
	printf(PanicLevel, &l.entry, format, args)
}

// Fatal outputs log with level FatalLevel.
func (l *Logger) Fatal(args ...interface{}) {
	printf(FatalLevel, &l.entry, "", args)
}

// Fatalf outputs log with level FatalLevel.
func (l *Logger) Fatalf(format string, args ...interface{}) {
	printf(FatalLevel, &l.entry, format, args)
}
