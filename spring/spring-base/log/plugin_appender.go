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
	"os"
)

func init() {
	RegisterPlugin("Null", PluginTypeAppender, (*NullAppender)(nil))
	RegisterPlugin("Console", PluginTypeAppender, (*ConsoleAppender)(nil))
	RegisterPlugin("File", PluginTypeAppender, (*FileAppender)(nil))
	RegisterPlugin("RollingFile", PluginTypeAppender, (*RollingFileAppender)(nil))
}

// Appender represents an output destination.
// Don't provide an asynchronous appender, because we have asynchronous logger.
type Appender interface {
	LifeCycle
	GetName() string
	GetLayout() Layout
	Append(e *Event)
}

var (
	_ Appender = (*NullAppender)(nil)
	_ Appender = (*ConsoleAppender)(nil)
	_ Appender = (*FileAppender)(nil)
	_ Appender = (*RollingFileAppender)(nil)
)

type BaseAppender struct {
	Name   string `PluginAttribute:"name"`
	Layout Layout `PluginElement:"Layout,default=PatternLayout"`
}

func (c *BaseAppender) Start() error             { return nil }
func (c *BaseAppender) Stop(ctx context.Context) {}
func (c *BaseAppender) GetName() string          { return c.Name }
func (c *BaseAppender) GetLayout() Layout        { return c.Layout }

// NullAppender is an Appender that ignores log events.
type NullAppender struct{}

func (c *NullAppender) Start() error             { return nil }
func (c *NullAppender) Stop(ctx context.Context) {}
func (c *NullAppender) GetName() string          { return "" }
func (c *NullAppender) GetLayout() Layout        { return nil }
func (c *NullAppender) Append(e *Event)          {}

// ConsoleAppender is an Appender that writing messages to os.Stdout.
type ConsoleAppender struct {
	BaseAppender
}

func (c *ConsoleAppender) Append(e *Event) {
	data, err := c.Layout.ToBytes(e)
	if err != nil {
		return
	}
	_, _ = os.Stdout.Write(data)
}

// FileAppender is an Appender writing messages to *os.File.
type FileAppender struct {
	BaseAppender
	writer   Writer
	FileName string `PluginAttribute:"fileName"`
}

func (c *FileAppender) Start() error {
	w, err := Writers.Get(c.FileName, func() (Writer, error) {
		return NewFileWriter(c.FileName)
	})
	if err != nil {
		return err
	}
	c.writer = w
	return nil
}

func (c *FileAppender) Stop(ctx context.Context) {
	Writers.Release(ctx, c.writer)
}

func (c *FileAppender) Append(e *Event) {
	data, err := c.Layout.ToBytes(e)
	if err != nil {
		return
	}
	_, _ = c.writer.Write(data)
}

type RollingFileAppender struct {
	BaseAppender
}

func (c *RollingFileAppender) Append(e *Event) {

}
