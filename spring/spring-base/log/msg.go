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
	Level Level
	Time  time.Time
	Ctx   context.Context
	Tag   string
	File  string
	Line  int
	Args  []interface{}
	Errno Errno
}

// newMessage 创建新的 *Message 对象。
func newMessage() *Message {
	return msgPool.Get().(*Message)
}

// Reuse 将 *Message 放回对象池，以便重用。
func (msg *Message) Reuse() {
	msg.reset()
	msgPool.Put(msg)
}

func (msg *Message) reset() {
	msg.Level = NoneLevel
	msg.Ctx = nil
	msg.Tag = ""
	msg.File = ""
	msg.Line = 0
	msg.Time = time.Time{}
	msg.Args = nil
	msg.Errno = nil
}
