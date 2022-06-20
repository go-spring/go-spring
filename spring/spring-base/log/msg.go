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
	"encoding/json"
	"fmt"
)

type Message interface {
	Text() string
}

type FormatMessage struct {
	format string
	args   []interface{}
}

func NewFormatMessage(format string, args []interface{}) Message {
	return &FormatMessage{
		format: format,
		args:   args,
	}
}

func (msg *FormatMessage) Text() string {
	if len(msg.args) == 1 {
		fn, ok := msg.args[0].(func() []interface{})
		if ok {
			msg.args = fn()
		}
	}
	if msg.format == "" {
		return fmt.Sprint(msg.args...)
	}
	return fmt.Sprintf(msg.format, msg.args...)
}

type JsonMessage struct {
	data interface{}
}

func NewJsonMessage(data interface{}) Message {
	return &JsonMessage{
		data: data,
	}
}

func (msg *JsonMessage) Text() string {
	b, err := json.Marshal(msg.data)
	if err != nil {
		return err.Error()
	}
	return string(b)
}
