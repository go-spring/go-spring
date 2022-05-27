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
	"time"
)

var defaultLoggerConfig = &LoggerConfig{
	Level:     InfoLevel,
	Appenders: []Appender{NewConsoleAppender(new(ConsoleAppenderConfig))},
}

func printf(level Level, e Entry, format string, args []interface{}) {
	config := e.Logger().config()
	if config == nil {
		config = defaultLoggerConfig
	}
	if config.Level > level {
		return
	}
	if len(args) == 1 {
		if fn, ok := args[0].(func() []interface{}); ok {
			args = fn()
		}
	}
	var text string
	if format == "" {
		text = fmt.Sprint(args...)
	} else {
		text = fmt.Sprintf(format, args...)
	}
	doPrint(config.Appenders, level, e, text)
}

func doPrint(appenders []Appender, level Level, e Entry, text string) {
	file, line, _ := Caller(e.Skip()+3, true)
	msg := &Message{
		level: level,
		time:  time.Now(),
		ctx:   e.Context(),
		tag:   e.Tag(),
		file:  file,
		line:  line,
		text:  text,
		errno: e.Errno(),
	}
	for _, appender := range appenders {
		appender.Append(msg)
	}
}
