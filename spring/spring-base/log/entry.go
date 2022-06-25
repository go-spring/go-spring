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

const (
	Undefined    = "_undef"
	RequestIn    = "_request_in"
	RequestOut   = "_request_out"
	HttpSuccess  = "_http_success"
	HttpFailure  = "_http_failure"
	GRPCSuccess  = "_grpc_success"
	GRPCFailure  = "_grpc_failure"
	MysqlSuccess = "_mysql_success"
	MysqlFailure = "_mysql_failure"
	RedisSuccess = "_redis_success"
	RedisFailure = "_redis_failure"
)

// Entry provides context, errno and tag about a log message.
type Entry interface {
	Tag() string
	Errno() Errno
	Context() context.Context
}

// publisher will drop a log message when the Filter method returns true.
type publisher interface {
	filter(level Level, e Entry, msg Message) Result
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

// Trace outputs log with level TraceLevel.
func (e SimpleEntry) Trace(args ...interface{}) *Event {
	return printf(e.pub, TraceLevel, e.skip, &e, "", args)
}

// Tracef outputs log with level TraceLevel.
func (e SimpleEntry) Tracef(format string, args ...interface{}) *Event {
	return printf(e.pub, TraceLevel, e.skip, &e, format, args)
}

// Debug outputs log with level DebugLevel.
func (e SimpleEntry) Debug(args ...interface{}) *Event {
	return printf(e.pub, DebugLevel, e.skip, &e, "", args)
}

// Debugf outputs log with level DebugLevel.
func (e SimpleEntry) Debugf(format string, args ...interface{}) *Event {
	return printf(e.pub, DebugLevel, e.skip, &e, format, args)
}

// Info outputs log with level InfoLevel.
func (e SimpleEntry) Info(args ...interface{}) *Event {
	return printf(e.pub, InfoLevel, e.skip, &e, "", args)
}

// Infof outputs log with level InfoLevel.
func (e SimpleEntry) Infof(format string, args ...interface{}) *Event {
	return printf(e.pub, InfoLevel, e.skip, &e, format, args)
}

// Warn outputs log with level WarnLevel.
func (e SimpleEntry) Warn(args ...interface{}) *Event {
	return printf(e.pub, WarnLevel, e.skip, &e, "", args)
}

// Warnf outputs log with level WarnLevel.
func (e SimpleEntry) Warnf(format string, args ...interface{}) *Event {
	return printf(e.pub, WarnLevel, e.skip, &e, format, args)
}

// Error outputs log with level ErrorLevel.
func (e SimpleEntry) Error(args ...interface{}) *Event {
	return printf(e.pub, ErrorLevel, e.skip, &e, "", args)
}

// Errorf outputs log with level ErrorLevel.
func (e SimpleEntry) Errorf(format string, args ...interface{}) *Event {
	return printf(e.pub, ErrorLevel, e.skip, &e, format, args)
}

// Panic outputs log with level PanicLevel.
func (e SimpleEntry) Panic(args ...interface{}) *Event {
	return printf(e.pub, PanicLevel, e.skip, &e, "", args)
}

// Panicf outputs log with level PanicLevel.
func (e SimpleEntry) Panicf(format string, args ...interface{}) *Event {
	return printf(e.pub, PanicLevel, e.skip, &e, format, args)
}

// Fatal outputs log with level FatalLevel.
func (e SimpleEntry) Fatal(args ...interface{}) *Event {
	return printf(e.pub, FatalLevel, e.skip, &e, "", args)
}

// Fatalf outputs log with level FatalLevel.
func (e SimpleEntry) Fatalf(format string, args ...interface{}) *Event {
	return printf(e.pub, FatalLevel, e.skip, &e, format, args)
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

// Trace outputs log with level TraceLevel.
func (e ContextEntry) Trace(args ...interface{}) *Event {
	return printf(e.pub, TraceLevel, e.skip, &e, "", args)
}

// Tracef outputs log with level TraceLevel.
func (e ContextEntry) Tracef(format string, args ...interface{}) *Event {
	return printf(e.pub, TraceLevel, e.skip, &e, format, args)
}

// Debug outputs log with level DebugLevel.
func (e ContextEntry) Debug(args ...interface{}) *Event {
	return printf(e.pub, DebugLevel, e.skip, &e, "", args)
}

// Debugf outputs log with level DebugLevel.
func (e ContextEntry) Debugf(format string, args ...interface{}) *Event {
	return printf(e.pub, DebugLevel, e.skip, &e, format, args)
}

// Info outputs log with level InfoLevel.
func (e ContextEntry) Info(args ...interface{}) *Event {
	return printf(e.pub, InfoLevel, e.skip, &e, "", args)
}

// Infof outputs log with level InfoLevel.
func (e ContextEntry) Infof(format string, args ...interface{}) *Event {
	return printf(e.pub, InfoLevel, e.skip, &e, format, args)
}

// Warn outputs log with level WarnLevel.
func (e ContextEntry) Warn(args ...interface{}) *Event {
	return printf(e.pub, WarnLevel, e.skip, &e, "", args)
}

// Warnf outputs log with level WarnLevel.
func (e ContextEntry) Warnf(format string, args ...interface{}) *Event {
	return printf(e.pub, WarnLevel, e.skip, &e, format, args)
}

// Error outputs log with level ErrorLevel.
func (e ContextEntry) Error(errno Errno, args ...interface{}) *Event {
	e.errno = errno
	return printf(e.pub, ErrorLevel, e.skip, &e, "", args)
}

// Errorf outputs log with level ErrorLevel.
func (e ContextEntry) Errorf(errno Errno, format string, args ...interface{}) *Event {
	e.errno = errno
	return printf(e.pub, ErrorLevel, e.skip, &e, format, args)
}

// Panic outputs log with level PanicLevel.
func (e ContextEntry) Panic(args ...interface{}) *Event {
	return printf(e.pub, PanicLevel, e.skip, &e, "", args)
}

// Panicf outputs log with level PanicLevel.
func (e ContextEntry) Panicf(format string, args ...interface{}) *Event {
	return printf(e.pub, PanicLevel, e.skip, &e, format, args)
}

// Fatal outputs log with level FatalLevel.
func (e ContextEntry) Fatal(args ...interface{}) *Event {
	return printf(e.pub, FatalLevel, e.skip, &e, "", args)
}

// Fatalf outputs log with level FatalLevel.
func (e ContextEntry) Fatalf(format string, args ...interface{}) *Event {
	return printf(e.pub, FatalLevel, e.skip, &e, format, args)
}

func printf(p publisher, level Level, skip int, e Entry, format string, args []interface{}) *Event {
	var msg Message
	if format == "" && len(args) == 1 {
		if t, ok := args[0].(Message); ok {
			msg = t
		}
	}
	if msg == nil {
		msg = NewFormattedMessage(format, args)
	}
	if ResultDeny == p.filter(level, e, msg) {
		return nil
	}
	file, line, _ := Caller(skip+2, true)
	event := &Event{
		entry: e,
		time:  time.Now(),
		msg:   msg,
		file:  file,
		line:  line,
		level: level,
	}
	p.publish(event)
	return event
}
