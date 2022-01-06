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
	"sync"
	"time"
)

// msgPool *Message 对象池。
var msgPool = sync.Pool{
	New: func() interface{} {
		return &Message{}
	},
}

// Message 定义日志消息。
type Message struct {
	time  time.Time
	ctx   context.Context
	tag   string
	file  string
	line  int
	args  []interface{}
	errno ErrNo
}

// newMessage 创建新的 *Message 对象。
func newMessage() *Message {
	return msgPool.Get().(*Message)
}

func (msg *Message) reset() {
	msg.ctx = nil
	msg.tag = ""
	msg.file = ""
	msg.line = 0
	msg.time = time.Time{}
	msg.args = msg.args[:0]
}

func (msg *Message) Ctx() context.Context {
	return msg.ctx
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

func (msg *Message) ErrNo() ErrNo {
	return msg.errno
}

// Reuse 将 *Message 放回对象池，以便重用。
func (msg *Message) Reuse() {
	msg.reset()
	msgPool.Put(msg)
}
