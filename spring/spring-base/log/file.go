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

	"github.com/go-spring/spring-base/util"
)

func init() {
	RegisterPlugin("SimpleFile", PluginTypeAppender, new(SimpleFileAppender))
}

// SimpleFileAppender is an Appender writing messages to *os.File.
type SimpleFileAppender struct {
	Name     string `PluginAttribute:"name"`
	FileName string `PluginAttribute:"fileName"`
	writer   Writer
}

func (c *SimpleFileAppender) Start() error {
	writer, err := NewWriter(c.FileName, func() (Writer, error) {
		return NewFileWriter(c.FileName)
	})
	if err != nil {
		return err
	}
	c.writer = writer
	return nil
}

func (c *SimpleFileAppender) Stop(ctx context.Context) {
	c.writer.Stop(ctx)
}

func (c *SimpleFileAppender) Append(e *Event) {
	strTime := e.Time().Format("2006-01-02T15:04:05.000")
	fileLine := util.Contract(fmt.Sprintf("%s:%d", e.File(), e.Line()), 48)
	data := fmt.Sprintf("[%s][%s][%s] %s\n", e.Level(), strTime, fileLine, e.Text())
	_, _ = c.writer.Write([]byte(data))
}
