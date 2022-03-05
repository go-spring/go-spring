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

	"github.com/go-spring/spring-base/clock"
)

var defaultLoggerConfig = &loggerConfig{
	level:     InfoLevel,
	appenders: []Appender{NewConsoleAppender(new(ConsoleAppenderConfig))},
}

func printf(level Level, e Entry, format string, args []interface{}) {
	config := e.Logger().getConfig()
	if config == nil {
		config = defaultLoggerConfig
	}
	if config.level > level {
		return
	}
	if len(args) == 1 {
		if fn, ok := args[0].(func() []interface{}); ok {
			args = fn()
		}
	}
	if format != "" {
		args = []interface{}{fmt.Sprintf(format, args...)}
	}
	doPrint(config.appenders, level, e, args)
}

func doPrint(appenders []Appender, level Level, e Entry, args []interface{}) {
	msg := new(Message)
	msg.Level = level
	msg.Args = args
	msg.Tag = e.Tag()
	msg.Ctx = e.Context()
	msg.Errno = e.Errno()
	ctx := msg.Ctx
	if ctx == nil {
		ctx = context.TODO()
	}
	msg.Time = clock.Now(ctx)
	msg.File, msg.Line, _ = Caller(e.Skip()+3, true)
	for _, appender := range appenders {
		appender.Append(msg)
	}
}
