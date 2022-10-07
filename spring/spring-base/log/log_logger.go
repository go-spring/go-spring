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
	level Level
}

// wrapperConfig atomic.Value 要求底层数据完全一致。
type wrapperConfig struct {
	config privateConfig
}

func newLogger(name string, level Level) *Logger {
	return &Logger{name: name, level: level}
}

// Name returns the logger's name.
func (l *Logger) Name() string {
	return l.name
}

func (l *Logger) config() privateConfig {
	return l.value.Load().(*wrapperConfig).config
}

func (l *Logger) reconfigure(config privateConfig) {
	l.value.Store(&wrapperConfig{config})
}

func (l *Logger) Level() Level {
	return l.config().getLevel()
}

func (l *Logger) Filter() Filter {
	return l.config().getFilter()
}

func (l *Logger) Appenders() []Appender {
	c := l.config()
	var appenders []Appender
	for _, ref := range c.getAppenders() {
		appenders = append(appenders, ref.appender)
	}
	return appenders
}

// WithSkip 创建包含 skip 信息的 Entry 。
func (l *Logger) WithSkip(n int) SimpleEntry {
	return SimpleEntry{pub: l.config(), skip: n}
}

// WithTag 创建包含 tag 信息的 Entry 。
func (l *Logger) WithTag(tag string) SimpleEntry {
	return SimpleEntry{pub: l.config(), tag: tag}
}

// WithContext 创建包含 context.Context 对象的 Entry 。
func (l *Logger) WithContext(ctx context.Context) ContextEntry {
	return ContextEntry{pub: l.config(), ctx: ctx}
}

func (l *Logger) enableLog(level Level) (privateConfig, bool) {
	c := l.config()
	s := c.getName()
	if len(s) != len(l.name) || s != l.name {
		if level < l.level {
			return c, false
		}
	}
	return c, true
}

// Trace outputs log with level TraceLevel.
func (l *Logger) Trace(args ...interface{}) *Event {
	c, ok := l.enableLog(TraceLevel)
	if !ok {
		return nil
	}
	return c.getEntry().WithSkip(1).Trace(args...)
}

// Tracef outputs log with level TraceLevel.
func (l *Logger) Tracef(format string, args ...interface{}) *Event {
	c, ok := l.enableLog(TraceLevel)
	if !ok {
		return nil
	}
	return c.getEntry().WithSkip(1).Tracef(format, args...)
}

// Tracew outputs log with level TraceLevel.
func (l *Logger) Tracew(fields ...Field) *Event {
	c, ok := l.enableLog(TraceLevel)
	if !ok {
		return nil
	}
	return c.getEntry().WithSkip(1).Tracew(fields...)
}

// Debug outputs log with level DebugLevel.
func (l *Logger) Debug(args ...interface{}) *Event {
	c, ok := l.enableLog(DebugLevel)
	if !ok {
		return nil
	}
	return c.getEntry().WithSkip(1).Debug(args...)
}

// Debugf outputs log with level DebugLevel.
func (l *Logger) Debugf(format string, args ...interface{}) *Event {
	c, ok := l.enableLog(DebugLevel)
	if !ok {
		return nil
	}
	return c.getEntry().WithSkip(1).Debugf(format, args...)
}

// Debugw outputs log with level DebugLevel.
func (l *Logger) Debugw(fields ...Field) *Event {
	c, ok := l.enableLog(DebugLevel)
	if !ok {
		return nil
	}
	return c.getEntry().WithSkip(1).Debugw(fields...)
}

// Info outputs log with level InfoLevel.
func (l *Logger) Info(args ...interface{}) *Event {
	c, ok := l.enableLog(InfoLevel)
	if !ok {
		return nil
	}
	return c.getEntry().WithSkip(1).Info(args...)
}

// Infof outputs log with level InfoLevel.
func (l *Logger) Infof(format string, args ...interface{}) *Event {
	c, ok := l.enableLog(InfoLevel)
	if !ok {
		return nil
	}
	return c.getEntry().WithSkip(1).Infof(format, args...)
}

// Infow outputs log with level InfoLevel.
func (l *Logger) Infow(fields ...Field) *Event {
	c, ok := l.enableLog(InfoLevel)
	if !ok {
		return nil
	}
	return c.getEntry().WithSkip(1).Infow(fields...)
}

// Warn outputs log with level WarnLevel.
func (l *Logger) Warn(args ...interface{}) *Event {
	c, ok := l.enableLog(WarnLevel)
	if !ok {
		return nil
	}
	return c.getEntry().WithSkip(1).Warn(args...)
}

// Warnf outputs log with level WarnLevel.
func (l *Logger) Warnf(format string, args ...interface{}) *Event {
	c, ok := l.enableLog(WarnLevel)
	if !ok {
		return nil
	}
	return c.getEntry().WithSkip(1).Warnf(format, args...)
}

// Warnw outputs log with level WarnLevel.
func (l *Logger) Warnw(fields ...Field) *Event {
	c, ok := l.enableLog(WarnLevel)
	if !ok {
		return nil
	}
	return c.getEntry().WithSkip(1).Warnw(fields...)
}

// Error outputs log with level ErrorLevel.
func (l *Logger) Error(args ...interface{}) *Event {
	c, ok := l.enableLog(ErrorLevel)
	if !ok {
		return nil
	}
	return c.getEntry().WithSkip(1).Error(args...)
}

// Errorf outputs log with level ErrorLevel.
func (l *Logger) Errorf(format string, args ...interface{}) *Event {
	c, ok := l.enableLog(ErrorLevel)
	if !ok {
		return nil
	}
	return c.getEntry().WithSkip(1).Errorf(format, args...)
}

// Errorw outputs log with level ErrorLevel.
func (l *Logger) Errorw(fields ...Field) *Event {
	c, ok := l.enableLog(ErrorLevel)
	if !ok {
		return nil
	}
	return c.getEntry().WithSkip(1).Errorw(fields...)
}

// Panic outputs log with level PanicLevel.
func (l *Logger) Panic(args ...interface{}) *Event {
	c, ok := l.enableLog(PanicLevel)
	if !ok {
		return nil
	}
	return c.getEntry().WithSkip(1).Panic(args...)
}

// Panicf outputs log with level PanicLevel.
func (l *Logger) Panicf(format string, args ...interface{}) *Event {
	c, ok := l.enableLog(PanicLevel)
	if !ok {
		return nil
	}
	return c.getEntry().WithSkip(1).Panicf(format, args...)
}

// Panicw outputs log with level PanicLevel.
func (l *Logger) Panicw(fields ...Field) *Event {
	c, ok := l.enableLog(PanicLevel)
	if !ok {
		return nil
	}
	return c.getEntry().WithSkip(1).Panicw(fields...)
}

// Fatal outputs log with level FatalLevel.
func (l *Logger) Fatal(args ...interface{}) *Event {
	c, ok := l.enableLog(FatalLevel)
	if !ok {
		return nil
	}
	return c.getEntry().WithSkip(1).Fatal(args...)
}

// Fatalf outputs log with level FatalLevel.
func (l *Logger) Fatalf(format string, args ...interface{}) *Event {
	c, ok := l.enableLog(FatalLevel)
	if !ok {
		return nil
	}
	return c.getEntry().WithSkip(1).Fatalf(format, args...)
}

// Fatalw outputs log with level FatalLevel.
func (l *Logger) Fatalw(fields ...Field) *Event {
	c, ok := l.enableLog(FatalLevel)
	if !ok {
		return nil
	}
	return c.getEntry().WithSkip(1).Fatalw(fields...)
}
