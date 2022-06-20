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
	RegisterPlugin("Console", PluginTypeAppender, (*ConsoleAppender)(nil))
}

// ConsoleAppender is an Appender writing messages to os.Stdout.
type ConsoleAppender struct {
	Name   string `PluginAttribute:"name"`
	Filter Filter `PluginElement:"Filter"`
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
	_, _ = fmt.Printf("[%s][%s][%s] %s\n", strLevel, strTime, fileLine, e.Text())
}
