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

	"github.com/go-spring/spring-base/log/queue"
)

var (
	empty = &emptyConfig{}
)

func init() {
	RegisterPlugin("Root", "Root", (*LoggerConfig)(nil))
	RegisterPlugin("Logger", "Logger", (*LoggerConfig)(nil))
	RegisterPlugin("AsyncRoot", "AsyncRoot", (*AsyncLoggerConfig)(nil))
	RegisterPlugin("AsyncLogger", "AsyncLogger", (*AsyncLoggerConfig)(nil))
	RegisterPlugin("AppenderRef", "AppenderRef", (*AppenderRef)(nil))
}

type PrivateConfig interface {
	publisher
	start() error
	stop(ctx context.Context)
	GetEntry() SimpleEntry
	GetParent() PrivateConfig
	GetName() string
	GetLevel() Level
	GetFilters() []Filter
	GetAppenders() []*AppenderRef
}

type emptyConfig struct {
	publisher
}

func (c *emptyConfig) Start() error                 { return nil }
func (c *emptyConfig) Stop(ctx context.Context)     {}
func (c *emptyConfig) GetEntry() SimpleEntry        { return SimpleEntry{} }
func (c *emptyConfig) GetParent() PrivateConfig     { return nil }
func (c *emptyConfig) GetName() string              { return "" }
func (c *emptyConfig) GetLevel() Level              { return NoneLevel }
func (c *emptyConfig) GetFilters() []Filter         { return nil }
func (c *emptyConfig) GetAppenders() []*AppenderRef { return nil }

type AppenderRef struct {
	appender Appender
	Ref      string   `PluginAttribute:"ref"`
	Level    Level    `PluginAttribute:"level,default=info"`
	Filters  []Filter `PluginElement:"Filter"`
}

func (r *AppenderRef) Append(e *Event) {
	if e.level < r.Level {
		return
	}
	for _, filter := range r.Filters {
		if filter.Filter(e.level, e.entry, e.msg) {
			return
		}
	}
	r.appender.Append(e)
}

type BaseLoggerConfig struct {
	entry        SimpleEntry
	parent       PrivateConfig
	Name         string         `PluginAttribute:"name"`
	Level        Level          `PluginAttribute:"level,default=info"`
	Additivity   bool           `PluginAttribute:"additivity,default=true"`
	Filters      []Filter       `PluginElement:"Filter"`
	AppenderRefs []*AppenderRef `PluginElement:"AppenderRef"`
}

func (c *BaseLoggerConfig) GetName() string {
	return c.Name
}

func (c *BaseLoggerConfig) GetEntry() SimpleEntry {
	return c.entry
}

func (c *BaseLoggerConfig) GetParent() PrivateConfig {
	return c.parent
}

func (c *BaseLoggerConfig) GetLevel() Level {
	return c.Level
}

func (c *BaseLoggerConfig) GetFilters() []Filter {
	return c.Filters
}

func (c *BaseLoggerConfig) GetAppenders() []*AppenderRef {
	return c.AppenderRefs
}

func (c *BaseLoggerConfig) callAppenders(e *Event) {
	for _, r := range c.AppenderRefs {
		r.Append(e)
	}
	if c.parent != nil && c.Additivity {
		c.parent.publish(e)
	}
}

func (c *BaseLoggerConfig) filter(level Level, e Entry, msg Message) bool {
	if level < c.Level {
		return true
	}
	for _, filter := range c.Filters {
		if filter.Filter(level, e, msg) {
			return true
		}
	}
	return false
}

type LoggerConfig struct {
	BaseLoggerConfig
}

func (c *LoggerConfig) start() error {
	c.entry = SimpleEntry{pub: c}
	return nil
}

func (c *LoggerConfig) stop(ctx context.Context) {

}

func (c *LoggerConfig) publish(e *Event) {
	c.callAppenders(e)
}

type AsyncLoggerConfig struct {
	BaseLoggerConfig
}

func (c *AsyncLoggerConfig) start() error {
	c.entry = SimpleEntry{pub: c}
	return nil
}

func (c *AsyncLoggerConfig) stop(ctx context.Context) {

}

type eventWrapper struct {
	c *AsyncLoggerConfig
	e *Event
}

func (w *eventWrapper) OnEvent() {
	w.c.callAppenders(w.e)
}

func (c *AsyncLoggerConfig) publish(e *Event) {
	queue.Instance().Publish(&eventWrapper{c: c, e: e})
}
