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

	"github.com/go-spring/spring-base/atomic"
)

// Message is an interface that its implementations can be converted to text.
type Message interface {
	Text() string
}

// OptimizedMessage uses atomic.Value to store formatted text. Tests show that
// you can use OptimizedMessage to improve the performance of the Text method
// when you call the Text method more than once.
type OptimizedMessage struct {
	text atomic.Value
}

func (msg *OptimizedMessage) Once(fn func() string) string {
	v := msg.text.Load()
	if v == nil {
		text := fn()
		msg.text.Store(text)
		return text
	}
	return v.(string)
}

// FormattedMessage can convert itself to text by fmt.Sprint or fmt.Sprintf.
type FormattedMessage struct {
	format string
	args   []interface{}
}

func NewFormattedMessage(format string, args []interface{}) Message {
	return &FormattedMessage{
		format: format,
		args:   args,
	}
}

func (msg *FormattedMessage) Text() string {
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

// JsonMessage can convert itself to text by json.Marshal.
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
		Status.Errorf("json encode %v return error %s", msg.data, err)
		return fmt.Sprint(msg.data)
	}
	return string(b)
}
