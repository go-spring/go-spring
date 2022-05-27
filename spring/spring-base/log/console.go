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
	"fmt"
	"strings"

	"github.com/go-spring/spring-base/color"
	"github.com/go-spring/spring-base/util"
)

func init() {
	RegisterAppenderFactory("ConsoleAppender", new(ConsoleAppenderFactory))
}

type ConsoleAppenderFactory struct{}

func (f *ConsoleAppenderFactory) NewAppenderConfig() AppenderConfig {
	return new(ConsoleAppenderConfig)
}

func (f *ConsoleAppenderFactory) NewAppender(config AppenderConfig) (Appender, error) {
	return NewConsoleAppender(config.(*ConsoleAppenderConfig)), nil
}

type ConsoleAppenderConfig struct {
	Name string `xml:"name,attr"`
}

func (c *ConsoleAppenderConfig) GetName() string {
	return c.Name
}

type ConsoleAppender struct {
	config *ConsoleAppenderConfig
}

func NewConsoleAppender(config *ConsoleAppenderConfig) *ConsoleAppender {
	return &ConsoleAppender{config: config}
}

func (c *ConsoleAppender) Append(msg *Message) {
	level := msg.Level()
	strLevel := strings.ToUpper(level.String())
	if level >= ErrorLevel {
		strLevel = color.Red.Sprint(strLevel)
	} else if level == WarnLevel {
		strLevel = color.Yellow.Sprint(strLevel)
	} else if level == TraceLevel {
		strLevel = color.Green.Sprint(strLevel)
	}
	strTime := msg.Time().Format("2006-01-02T15:04:05.000")
	fileLine := util.Contract(fmt.Sprintf("%s:%d", msg.File(), msg.Line()), 48)
	_, _ = fmt.Printf("[%s][%s][%s] %s\n", strLevel, strTime, fileLine, msg.text)
}
