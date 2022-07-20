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

	"github.com/go-spring/spring-base/atomic"
)

func init() {
	RegisterPlugin("Null", PluginTypeAppender, (*NullAppender)(nil))
	RegisterPlugin("CountingNoOp", PluginTypeAppender, (*CountingNoOpAppender)(nil))
	RegisterPlugin("Console", PluginTypeAppender, (*ConsoleAppender)(nil))
	RegisterPlugin("File", PluginTypeAppender, new(FileAppender))
	RegisterPlugin("RollingFile", PluginTypeAppender, new(RollingFileAppender))
}

// Appender represents an output destination. Do not provide an asynchronous
// appender, because we have asynchronous logger.
type Appender interface {
	LifeCycle
	GetName() string
	GetLayout() Layout
	Append(e *Event)
}

type BaseAppender struct {
	Name   string `PluginAttribute:"name"`
	Layout Layout `PluginElement:"Layout,default=DefaultLayout"`
}

func (c *BaseAppender) Start() error             { return nil }
func (c *BaseAppender) Stop(ctx context.Context) {}
func (c *BaseAppender) GetName() string          { return c.Name }
func (c *BaseAppender) GetLayout() Layout        { return c.Layout }

// NullAppender is an Appender that ignores log events.
type NullAppender struct{}

func (c *NullAppender) Start() error             { return nil }
func (c *NullAppender) Stop(ctx context.Context) {}
func (c *NullAppender) Append(e *Event)          {}

// CountingNoOpAppender is a no-operation Appender that counts events.
type CountingNoOpAppender struct {
	count atomic.Int64
}

func (c *CountingNoOpAppender) Start() error             { return nil }
func (c *CountingNoOpAppender) Stop(ctx context.Context) {}
func (c *CountingNoOpAppender) Count() int64             { return c.count.Load() }
func (c *CountingNoOpAppender) Append(e *Event)          { c.count.Add(1) }

// ConsoleAppender is an Appender that writing messages to os.Stdout.
type ConsoleAppender struct {
	BaseAppender
}

func (c *ConsoleAppender) Append(e *Event) {
	data, err := c.Layout.ToBytes(e)
	if err != nil {
		Status.Errorf("append error %s", err.Error())
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
		Status.Errorf("append error %s", err.Error())
		return
	}
	_, err = c.writer.Write(data)
	if err != nil {
		Status.Errorf("append error %s", err.Error())
	}
}

type RollingFileAppender struct {
	BaseAppender
}

func (c *RollingFileAppender) Append(e *Event) {

}
