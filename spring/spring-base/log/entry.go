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
	"time"
)

// Entry provides context, errno and tag about a log message.
type Entry interface {
	Tag() string
	Errno() Errno
	Context() context.Context
}

// publisher will drop a log message when the filter method returns true.
type publisher interface {
	filter(level Level, e Entry, fields []Field) Result
	publish(e *Event)
}

// SimpleEntry is an Entry implementation that has no context.
type SimpleEntry struct {
	pub  publisher
	tag  string
	skip int
}

func (e *SimpleEntry) Tag() string {
	return e.tag
}

func (e *SimpleEntry) Errno() Errno {
	return nil
}

func (e *SimpleEntry) Context() context.Context {
	return nil
}

func (e SimpleEntry) WithSkip(n int) SimpleEntry {
	e.skip = n
	return e
}

func (e SimpleEntry) WithTag(tag string) SimpleEntry {
	e.tag = tag
	return e
}

func (e SimpleEntry) WithContext(ctx context.Context) ContextEntry {
	return ContextEntry{
		pub:  e.pub,
		skip: e.skip,
		tag:  e.tag,
		ctx:  ctx,
	}
}

func (e SimpleEntry) Sugar() SugarSimpleEntry {
	return SugarSimpleEntry{e: e}
}

// Tracew outputs log with level TraceLevel.
func (e SimpleEntry) Tracew(fields ...Field) *Event {
	return publish(e.pub, TraceLevel, e.skip, &e, fields)
}

// Debugw outputs log with level DebugLevel.
func (e SimpleEntry) Debugw(fields ...Field) *Event {
	return publish(e.pub, DebugLevel, e.skip, &e, fields)
}

// Infow outputs log with level InfoLevel.
func (e SimpleEntry) Infow(fields ...Field) *Event {
	return publish(e.pub, InfoLevel, e.skip, &e, fields)
}

// Warnw outputs log with level WarnLevel.
func (e SimpleEntry) Warnw(fields ...Field) *Event {
	return publish(e.pub, WarnLevel, e.skip, &e, fields)
}

// Errorw outputs log with level ErrorLevel.
func (e SimpleEntry) Errorw(fields ...Field) *Event {
	return publish(e.pub, ErrorLevel, e.skip, &e, fields)
}

// Panicw outputs log with level PanicLevel.
func (e SimpleEntry) Panicw(fields ...Field) *Event {
	return publish(e.pub, PanicLevel, e.skip, &e, fields)
}

// Fatalw outputs log with level FatalLevel.
func (e SimpleEntry) Fatalw(fields ...Field) *Event {
	return publish(e.pub, FatalLevel, e.skip, &e, fields)
}

// ContextEntry is an Entry implementation that has context and errno.
type ContextEntry struct {
	ctx   context.Context
	errno Errno
	pub   publisher
	tag   string
	skip  int
}

func (e *ContextEntry) Tag() string {
	return e.tag
}

func (e *ContextEntry) Errno() Errno {
	return e.errno
}

func (e *ContextEntry) Context() context.Context {
	return e.ctx
}

func (e ContextEntry) WithSkip(n int) ContextEntry {
	e.skip = n
	return e
}

func (e ContextEntry) WithTag(tag string) ContextEntry {
	e.tag = tag
	return e
}

func (e ContextEntry) Sugar() SugarContextEntry {
	return SugarContextEntry{e: e}
}

// Tracew outputs log with level TraceLevel.
func (e ContextEntry) Tracew(fields ...Field) *Event {
	return publish(e.pub, TraceLevel, e.skip, &e, fields)
}

// Debugw outputs log with level DebugLevel.
func (e ContextEntry) Debugw(fields ...Field) *Event {
	return publish(e.pub, DebugLevel, e.skip, &e, fields)
}

// Infow outputs log with level InfoLevel.
func (e ContextEntry) Infow(fields ...Field) *Event {
	return publish(e.pub, InfoLevel, e.skip, &e, fields)
}

// Warnw outputs log with level WarnLevel.
func (e ContextEntry) Warnw(fields ...Field) *Event {
	return publish(e.pub, WarnLevel, e.skip, &e, fields)
}

// Errorw outputs log with level ErrorLevel.
func (e ContextEntry) Errorw(errno Errno, fields ...Field) *Event {
	e.errno = errno
	return publish(e.pub, ErrorLevel, e.skip, &e, fields)
}

// Panicw outputs log with level PanicLevel.
func (e ContextEntry) Panicw(fields ...Field) *Event {
	return publish(e.pub, PanicLevel, e.skip, &e, fields)
}

// Fatalw outputs log with level FatalLevel.
func (e ContextEntry) Fatalw(fields ...Field) *Event {
	return publish(e.pub, FatalLevel, e.skip, &e, fields)
}

func publish(p publisher, level Level, skip int, e Entry, fields []Field) *Event {
	if ResultDeny == p.filter(level, e, fields) {
		return nil
	}
	file, line, _ := Caller(skip+2, true)
	event := &Event{
		entry:  e,
		time:   time.Now(),
		file:   file,
		line:   line,
		level:  level,
		fields: fields,
	}
	p.publish(event)
	return event
}
