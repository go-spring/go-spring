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
	"fmt"

	"github.com/go-spring/spring-base/clock"
	"github.com/go-spring/spring-base/log/internal"
	"github.com/go-spring/spring-base/util"
)

type T = func() []interface{}

func W(fn func() []Field) Field {
	return Field{Key: "", Val: funcValue(fn)}
}

type funcValue func() []Field

func (v funcValue) Encode(enc Encoder) error {
	panic(util.ForbiddenMethod)
}

// publisher will drop a log message when the filter method returns true.
type publisher interface {
	enableLevel(level Level) bool
	publish(e *Event)
}

// Entry is an Entry implementation that has context and errno.
type Entry struct {
	ctx  context.Context
	pub  publisher
	tag  string
	skip int
}

func (e *Entry) WithSkip(n int) *Entry {
	e.skip = n
	return e
}

func (e *Entry) WithTag(tag string) *Entry {
	e.tag = tag
	return e
}

func (e *Entry) WithContext(ctx context.Context) *Entry {
	e.ctx = ctx
	return e
}

// Trace outputs log with level TraceLevel.
func (e *Entry) Trace(args ...interface{}) *Event {
	return publish(e.pub, TraceLevel, e.skip, e, "", args, nil)
}

// Tracef outputs log with level TraceLevel.
func (e *Entry) Tracef(format string, args ...interface{}) *Event {
	return publish(e.pub, TraceLevel, e.skip, e, format, args, nil)
}

// Tracew outputs log with level TraceLevel.
func (e *Entry) Tracew(fields ...Field) *Event {
	return publish(e.pub, TraceLevel, e.skip, e, "", nil, fields)
}

// Debug outputs log with level DebugLevel.
func (e *Entry) Debug(args ...interface{}) *Event {
	return publish(e.pub, DebugLevel, e.skip, e, "", args, nil)
}

// Debugf outputs log with level DebugLevel.
func (e *Entry) Debugf(format string, args ...interface{}) *Event {
	return publish(e.pub, DebugLevel, e.skip, e, format, args, nil)
}

// Debugw outputs log with level DebugLevel.
func (e *Entry) Debugw(fields ...Field) *Event {
	return publish(e.pub, DebugLevel, e.skip, e, "", nil, fields)
}

// Info outputs log with level InfoLevel.
func (e *Entry) Info(args ...interface{}) *Event {
	return publish(e.pub, InfoLevel, e.skip, e, "", args, nil)
}

// Infof outputs log with level InfoLevel.
func (e *Entry) Infof(format string, args ...interface{}) *Event {
	return publish(e.pub, InfoLevel, e.skip, e, format, args, nil)
}

// Infow outputs log with level InfoLevel.
func (e *Entry) Infow(fields ...Field) *Event {
	return publish(e.pub, InfoLevel, e.skip, e, "", nil, fields)
}

// Warn outputs log with level WarnLevel.
func (e *Entry) Warn(args ...interface{}) *Event {
	return publish(e.pub, WarnLevel, e.skip, e, "", args, nil)
}

// Warnf outputs log with level WarnLevel.
func (e *Entry) Warnf(format string, args ...interface{}) *Event {
	return publish(e.pub, WarnLevel, e.skip, e, format, args, nil)
}

// Warnw outputs log with level WarnLevel.
func (e *Entry) Warnw(fields ...Field) *Event {
	return publish(e.pub, WarnLevel, e.skip, e, "", nil, fields)
}

// Error outputs log with level ErrorLevel.
func (e *Entry) Error(args ...interface{}) *Event {
	return publish(e.pub, ErrorLevel, e.skip, e, "", args, nil)
}

// Errorf outputs log with level ErrorLevel.
func (e *Entry) Errorf(format string, args ...interface{}) *Event {
	return publish(e.pub, ErrorLevel, e.skip, e, format, args, nil)
}

// Errorw outputs log with level ErrorLevel.
func (e *Entry) Errorw(fields ...Field) *Event {
	return publish(e.pub, ErrorLevel, e.skip, e, "", nil, fields)
}

// Panic outputs log with level PanicLevel.
func (e *Entry) Panic(args ...interface{}) *Event {
	return publish(e.pub, PanicLevel, e.skip, e, "", args, nil)
}

// Panicf outputs log with level PanicLevel.
func (e *Entry) Panicf(format string, args ...interface{}) *Event {
	return publish(e.pub, PanicLevel, e.skip, e, format, args, nil)
}

// Panicw outputs log with level PanicLevel.
func (e *Entry) Panicw(fields ...Field) *Event {
	return publish(e.pub, PanicLevel, e.skip, e, "", nil, fields)
}

// Fatal outputs log with level FatalLevel.
func (e *Entry) Fatal(args ...interface{}) *Event {
	return publish(e.pub, FatalLevel, e.skip, e, "", args, nil)
}

// Fatalf outputs log with level FatalLevel.
func (e *Entry) Fatalf(format string, args ...interface{}) *Event {
	return publish(e.pub, FatalLevel, e.skip, e, format, args, nil)
}

// Fatalw outputs log with level FatalLevel.
func (e *Entry) Fatalw(fields ...Field) *Event {
	return publish(e.pub, FatalLevel, e.skip, e, "", nil, fields)
}

func getMessage(format string, args []interface{}) string {
	if len(args) == 0 {
		return format
	}
	if len(args) == 1 {
		fn, ok := args[0].(func() []interface{})
		if ok {
			args = fn()
		}
	}
	if format == "" {
		return fmt.Sprint(args...)
	}
	return fmt.Sprintf(format, args...)
}

func publish(p publisher, level Level, skip int, e *Entry, format string, args []interface{}, fields []Field) *Event {
	if !p.enableLevel(level) {
		return nil
	}
	if len(fields) == 1 {
		v, ok := fields[0].Val.(funcValue)
		if ok {
			fields = v()
		}
	}
	message := getMessage(format, args)
	file, line, _ := internal.Caller(skip+2, true)
	event := &Event{
		Tag:     e.tag,
		Time:    clock.Now(e.ctx),
		Context: e.ctx,
		File:    file,
		Line:    line,
		Level:   level,
		Fields:  fields,
		Message: message,
	}
	p.publish(event)
	return event
}
