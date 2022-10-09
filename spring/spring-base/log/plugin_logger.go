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
	getName() string
	getLevel() Level
	getAppenders() []*AppenderRef
}

// AppenderRef is a reference to an Appender.
type AppenderRef struct {
	appender Appender
	Ref      string `PluginAttribute:"ref"`
	Filter   Filter `PluginElement:"Filter"`
	Level    Level  `PluginAttribute:"level,default=none"`
}

func (r *AppenderRef) Append(e *Event) {
	if r.Level != NoneLevel && e.Level < r.Level {
		return
	}
	if r.Filter != nil && ResultDeny == r.Filter.Filter(e) {
		return
	}
	r.appender.Append(e)
}

// baseLoggerConfig is the base of loggerConfig and asyncLoggerConfig.
type baseLoggerConfig struct {
	root         privateConfig
	Name         string         `PluginAttribute:"name"`
	AppenderRefs []*AppenderRef `PluginElement:"AppenderRef"`
	Level        Level          `PluginAttribute:"level,default=info"`
	Additivity   bool           `PluginAttribute:"additivity,default=true"`
}

func (c *baseLoggerConfig) getName() string {
	return c.Name
}

func (c *baseLoggerConfig) getLevel() Level {
	return c.Level
}

func (c *baseLoggerConfig) getAppenders() []*AppenderRef {
	return c.AppenderRefs
}

// filter returns whether the event should be logged.
func (c *baseLoggerConfig) enableLevel(level Level) bool {
	return level >= c.Level
}

// callAppenders calls all the appenders inherited from the hierarchy circumventing.
func (c *baseLoggerConfig) callAppenders(e *Event) {
	for _, r := range c.AppenderRefs {
		r.Append(e)
	}
	if c.root != nil && c.Additivity {
		c.root.logEvent(e)
	}
}

// logEvent is used only for parent logging events.
func (c *baseLoggerConfig) logEvent(e *Event) {
	if !c.enableLevel(e.Level) {
		return
	}
	c.callAppenders(e)
}

// loggerConfig publishes events synchronously.
type loggerConfig struct {
	baseLoggerConfig
}

func (c *loggerConfig) publish(e *Event) {
	c.callAppenders(e)
}

// asyncLoggerConfig publishes events synchronously.
type asyncLoggerConfig struct {
	baseLoggerConfig
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
