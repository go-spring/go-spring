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
	"github.com/go-spring/spring-base/log/queue"
)

var empty privateConfig = &emptyConfig{}

func init() {
	RegisterPlugin("Root", "Root", (*loggerConfig)(nil))
	RegisterPlugin("Logger", "Logger", (*loggerConfig)(nil))
	RegisterPlugin("AsyncRoot", "AsyncRoot", (*asyncLoggerConfig)(nil))
	RegisterPlugin("AsyncLogger", "AsyncLogger", (*asyncLoggerConfig)(nil))
	RegisterPlugin("AppenderRef", "AppenderRef", (*AppenderRef)(nil))
}

// privateConfig is the inner Logger.
type privateConfig interface {
	publisher
	logEvent(e *Event)
	getParent() privateConfig
	getEntry() SimpleEntry
	getName() string
	getLevel() Level
	getFilter() Filter
	getAppenders() []*AppenderRef
}

type emptyConfig struct {
	publisher
}

func (c *emptyConfig) logEvent(e *Event)            {}
func (c *emptyConfig) getParent() privateConfig     { return nil }
func (c *emptyConfig) getEntry() SimpleEntry        { return SimpleEntry{} }
func (c *emptyConfig) getName() string              { return "" }
func (c *emptyConfig) getLevel() Level              { return OffLevel }
func (c *emptyConfig) getFilter() Filter            { return nil }
func (c *emptyConfig) getAppenders() []*AppenderRef { return nil }

// AppenderRef is a reference to an Appender.
type AppenderRef struct {
	appender Appender
	Ref      string `PluginAttribute:"ref"`
	Filter   Filter `PluginElement:"Filter"`
	Level    Level  `PluginAttribute:"level,default=none"`
}

func (r *AppenderRef) Append(e *Event) {
	if r.Level != NoneLevel && e.level < r.Level {
		return
	}
	if r.Filter != nil && ResultDeny == r.Filter.Filter(e.level, e.entry, e.msg) {
		return
	}
	r.appender.Append(e)
}

// baseLoggerConfig is the base of loggerConfig and asyncLoggerConfig.
type baseLoggerConfig struct {
	parent       privateConfig
	entry        SimpleEntry
	Name         string         `PluginAttribute:"name"`
	Filter       Filter         `PluginElement:"Filter"`
	AppenderRefs []*AppenderRef `PluginElement:"AppenderRef"`
	Level        Level          `PluginAttribute:"level,default=none"`
	Additivity   bool           `PluginAttribute:"additivity,default=true"`
}

func (c *baseLoggerConfig) getParent() privateConfig {
	return c.parent
}

func (c *baseLoggerConfig) getEntry() SimpleEntry {
	return c.entry
}

func (c *baseLoggerConfig) getName() string {
	return c.Name
}

func (c *baseLoggerConfig) getLevel() Level {
	return c.Level
}

func (c *baseLoggerConfig) getFilter() Filter {
	return c.Filter
}

func (c *baseLoggerConfig) getAppenders() []*AppenderRef {
	return c.AppenderRefs
}

// logEvent is used only for parent logging events.
func (c *baseLoggerConfig) logEvent(e *Event) {
	if ResultDeny != c.filter(e.level, e.entry, e.msg) {
		c.callAppenders(e)
	}
}

// filter returns whether the event should be logged.
func (c *baseLoggerConfig) filter(level Level, e Entry, msg Message) Result {
	if c.Filter != nil && ResultDeny == c.Filter.Filter(level, e, msg) {
		return ResultDeny
	}
	if c.Level != NoneLevel && level < c.Level {
		return ResultDeny
	}
	return ResultAccept
}

// callAppenders calls all the appenders inherited from the hierarchy circumventing.
func (c *baseLoggerConfig) callAppenders(e *Event) {
	for _, r := range c.AppenderRefs {
		r.Append(e)
	}
	if c.parent != nil && c.Additivity {
		c.parent.logEvent(e)
	}
}

// loggerConfig publishes events synchronously.
type loggerConfig struct {
	baseLoggerConfig
}

func (c *loggerConfig) Init() error {
	c.entry = SimpleEntry{pub: c}
	return nil
}

func (c *loggerConfig) publish(e *Event) {
	c.callAppenders(e)
}

// asyncLoggerConfig publishes events synchronously.
type asyncLoggerConfig struct {
	baseLoggerConfig
}

func (c *asyncLoggerConfig) Init() error {
	c.entry = SimpleEntry{pub: c}
	return nil
}

type eventWrapper struct {
	c *asyncLoggerConfig
	e *Event
}

func (w *eventWrapper) OnEvent() {
	w.c.callAppenders(w.e)
}

// publish pushes events into the queue and these events will consumed by other
// goroutine, so the current goroutine will not be blocked.
func (c *asyncLoggerConfig) publish(e *Event) {
	queue.Publish(&eventWrapper{c: c, e: e})
}
