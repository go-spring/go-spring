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
	"strings"

	"github.com/go-spring/spring-base/atomic"
	"github.com/go-spring/spring-base/color"
	"github.com/go-spring/spring-base/util"
)

func init() {
	RegisterPlugin("Null", PluginTypeAppender, (*NullAppender)(nil))
	RegisterPlugin("CountingNoOp", PluginTypeAppender, (*CountingNoOpAppender)(nil))
	RegisterPlugin("Console", PluginTypeAppender, (*ConsoleAppender)(nil))
	RegisterPlugin("File", PluginTypeAppender, new(FileAppender))
	RegisterPlugin("RollingFile", PluginTypeAppender, new(RollingFileAppender))
}

// NullAppender is an Appender that ignores log events.
type NullAppender struct{}

func (c *NullAppender) Append(e *Event) {}

// CountingNoOpAppender is a no-operation Appender that counts events.
type CountingNoOpAppender struct {
	count atomic.Int64
}

func (c *CountingNoOpAppender) Count() int64 {
	return c.count.Load()
}

func (c *CountingNoOpAppender) Append(e *Event) {
	c.count.Add(1)
}

// ConsoleAppender is an Appender that writing messages to os.Stdout.
type ConsoleAppender struct {
	Filter Filter `PluginElement:"Filter"`
	Name   string `PluginAttribute:"name"`
}

func (c *ConsoleAppender) Append(e *Event) {
	level := e.Level()
	strLevel := strings.ToUpper(level.String())
	if level >= ErrorLevel {
		strLevel = color.Red.Sprint(strLevel)
	} else if level == WarnLevel {
		strLevel = color.Yellow.Sprint(strLevel)
	} else if level <= DebugLevel {
		strLevel = color.Green.Sprint(strLevel)
	}
	strTime := e.Time().Format("2006-01-02T15:04:05.000")
	fileLine := util.Contract(fmt.Sprintf("%s:%d", e.File(), e.Line()), 48)
	_, _ = fmt.Printf("[%s][%s][%s] %s\n", strLevel, strTime, fileLine, e.Msg().Text())
}

// FileAppender is an Appender writing messages to *os.File.
type FileAppender struct {
	writer   Writer
	Name     string `PluginAttribute:"name"`
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
	strTime := e.Time().Format("2006-01-02T15:04:05.000")
	fileLine := util.Contract(fmt.Sprintf("%s:%d", e.File(), e.Line()), 48)
	data := fmt.Sprintf("[%s][%s][%s] %s\n", e.Level(), strTime, fileLine, e.Msg().Text())
	_, err := c.writer.Write([]byte(data))
	if err != nil {
		Status.Errorf("write log event to file %s error %s", c.FileName, err.Error())
	}
}

type RollingFileAppender struct {
}

func (c *RollingFileAppender) Append(e *Event) {

}
