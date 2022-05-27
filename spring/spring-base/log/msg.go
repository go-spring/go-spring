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
	"time"
)

// Message 定义日志消息。
type Message struct {
	time  time.Time
	ctx   context.Context
	errno Errno
	text  string
	tag   string
	file  string
	line  int
	level Level
}

func (msg *Message) Level() Level {
	return msg.level
}

func (msg *Message) Tag() string {
	return msg.tag
}

func (msg *Message) File() string {
	return msg.file
}

func (msg *Message) Line() int {
	return msg.line
}

func (msg *Message) Time() time.Time {
	return msg.time
}

func (msg *Message) Text() string {
	return msg.text
}

func (msg *Message) Errno() Errno {
	return msg.errno
}

func (msg *Message) Context() context.Context {
	return msg.ctx
}
