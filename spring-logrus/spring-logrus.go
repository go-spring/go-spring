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

package SpringLogrus

import (
	"bytes"
	"strings"
	"time"
	"fmt"
	"github.com/sirupsen/logrus"
	"github.com/didi/go-spring/spring-logger"
)

type NullOutput struct {
	Cached  [][]byte
	Flushed bool
}

func NewNullOutput() *NullOutput {
	return &NullOutput{
		Cached: make([][]byte, 0),
	}
}

func (output *NullOutput) Write(p []byte) (int, error) {
	if !output.Flushed {
		output.Cached = append(output.Cached, p)
	}
	return 0, nil
}

func (output *NullOutput) Output(Appenders map[string]SpringLogger.LoggerAppender) {
	output.Flushed = true
	for i := range output.Cached {
		for _, appender := range Appenders {
			appender.Write(output.Cached[i])
		}
	}
	output.Cached = nil
}

//
// 日志内容格式化器
//
type TextFormatter struct {
}

func (f *TextFormatter) Format(entry *logrus.Entry) ([]byte, error) {

	b := &bytes.Buffer{}

	b.WriteByte('[')
	b.WriteString(strings.ToUpper(entry.Level.String()))
	b.WriteByte(']')

	b.WriteByte('[')
	b.WriteString(entry.Time.Format(time.RFC3339))
	b.WriteByte(']')

	if entry.Caller != nil {
		b.WriteByte('[')
		b.WriteString(fmt.Sprintf("%s:%d", entry.Caller.File, entry.Caller.Line))
		b.WriteByte(']')
	}

	f.appendValue(b, entry.Message)

	b.WriteByte('\n')
	return b.Bytes(), nil
}

func (f *TextFormatter) appendValue(b *bytes.Buffer, value interface{}) {
	stringVal, ok := value.(string)
	if !ok {
		stringVal = fmt.Sprint(value)
	}
	b.WriteByte(' ')
	b.WriteString(stringVal)
}

//
// logrus 扩展
//
type SpringLogrusHook struct {
	Level     logrus.Level
	Formatter logrus.Formatter
	Appender  SpringLogger.LoggerAppender
}

func NewSpringLogrusHook(appender SpringLogger.LoggerAppender, level logrus.Level) *SpringLogrusHook {
	return &SpringLogrusHook{
		Level:     level,
		Appender:  appender,
		Formatter: &TextFormatter{},
	}
}

func (hook *SpringLogrusHook) Levels() []logrus.Level {
	levels := make([]logrus.Level, 0)
	for _, level := range logrus.AllLevels {
		if hook.Level >= level {
			levels = append(levels, level)
		}
	}
	return levels
}

func (hook *SpringLogrusHook) Fire(entry *logrus.Entry) error {
	content, err := hook.Formatter.Format(entry)
	_, err = hook.Appender.Write(content)
	return err
}
