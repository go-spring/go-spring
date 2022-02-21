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
	"bytes"
	"fmt"
	"strings"

	"github.com/go-spring/spring-base/atomic"
	"github.com/go-spring/spring-base/cast"
	"github.com/go-spring/spring-base/color"
	"github.com/go-spring/spring-base/util"
)

// Console 控制台日志输出。
var Console = newConsole()

type console struct {
	level atomic.Int32
}

func newConsole() *console {
	c := &console{}
	c.SetLevel(InfoLevel)
	return c
}

func (c *console) Level() Level {
	return Level(c.level.Load())
}

func (c *console) SetLevel(level Level) {
	c.level.Store(int32(level))
}

func (c *console) Print(msg *Message) {
	defer func() { msg.Reuse() }()
	level := msg.Level()
	strLevel := strings.ToUpper(level.String())
	if level >= ErrorLevel {
		strLevel = color.Red.Sprint(strLevel)
	} else if level == WarnLevel {
		strLevel = color.Yellow.Sprint(strLevel)
	} else if level == TraceLevel {
		strLevel = color.Green.Sprint(strLevel)
	}
	var buf bytes.Buffer
	for _, a := range msg.Args() {
		buf.WriteString(cast.ToString(a))
	}
	strTime := msg.Time().Format("2006-01-02T15:04:05.000")
	fileLine := util.Contract(fmt.Sprintf("%s:%d", msg.File(), msg.Line()), 48)
	_, _ = fmt.Printf("[%s][%s][%s] %s\n", strLevel, strTime, fileLine, buf.String())
}
