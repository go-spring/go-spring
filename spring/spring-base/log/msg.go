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
	level Level
	time  time.Time
	ctx   context.Context
	tag   string
	file  string
	line  int
	args  []interface{}
	errno Errno
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

func (msg *Message) Args() []interface{} {
	return msg.args
}

func (msg *Message) Errno() Errno {
	return msg.errno
}

func (msg *Message) Context() context.Context {
	return msg.ctx
}

type MessageBuilder struct {
	Level Level
	Time  time.Time
	Ctx   context.Context
	Tag   string
	File  string
	Line  int
	Args  []interface{}
	Errno Errno
}

func NewMessageBuilder() *MessageBuilder {
	return &MessageBuilder{}
}

func (b *MessageBuilder) WithLevel(level Level) *MessageBuilder {
	b.Level = level
	return b
}

func (b *MessageBuilder) WithTag(tag string) *MessageBuilder {
	b.Tag = tag
	return b
}

func (b *MessageBuilder) WithFile(file string) *MessageBuilder {
	b.File = file
	return b
}

func (b *MessageBuilder) WithLine(line int) *MessageBuilder {
	b.Line = line
	return b
}

func (b *MessageBuilder) WithTime(time time.Time) *MessageBuilder {
	b.Time = time
	return b
}

func (b *MessageBuilder) WithArgs(args []interface{}) *MessageBuilder {
	b.Args = args
	return b
}

func (b *MessageBuilder) WithErrno(errno Errno) *MessageBuilder {
	b.Errno = errno
	return b
}

func (b *MessageBuilder) WithContext(ctx context.Context) *MessageBuilder {
	b.Ctx = ctx
	return b
}

func (b *MessageBuilder) Build() *Message {
	return &Message{
		level: b.Level,
		time:  b.Time,
		ctx:   b.Ctx,
		tag:   b.Tag,
		file:  b.File,
		line:  b.Line,
		args:  b.Args,
		errno: b.Errno,
	}
}
