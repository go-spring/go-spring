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

package log_test

import (
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"github.com/go-spring/spring-base/assert"
	"github.com/go-spring/spring-base/atomic"
	"github.com/go-spring/spring-base/log"
	"github.com/go-spring/spring-base/util"
)

type FormattedMessage struct {
	format string
	args   []interface{}
	text   atomic.Value
}

func (msg *FormattedMessage) gen() string {
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

func (msg *FormattedMessage) Text() string {
	v := msg.text.Load()
	if v == nil {
		text := msg.gen()
		msg.text.Store(text)
		return text
	}
	return v.(string)
}

type OptimizedFormattedMessage struct {
	log.OptimizedMessage
	format string
	args   []interface{}
}

func (msg *OptimizedFormattedMessage) Text() string {
	return msg.Once(func() string {
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
	})
}

func BenchmarkFormattedMessage(b *testing.B) {

	testcases := []struct {
		format string
		args   []interface{}
		expect string
	}{
		{
			args:   []interface{}{"a", "%", "b"},
			expect: "a%b",
		},
		{
			args: []interface{}{
				func() []interface{} {
					return util.T("a", "%", "b")
				},
			},
			expect: "a%b",
		},
		{
			format: "%s%%%s",
			args:   []interface{}{"a", "b"},
			expect: "a%b",
		},
		{
			format: "%s%%%s",
			args: []interface{}{
				func() []interface{} {
					return util.T("a", "b")
				},
			},
			expect: "a%b",
		},
	}

	for _, n := range []int{1, 2, 3} {

		start := time.Now()
		for i := 0; i < b.N; i++ {
			for _, c := range testcases {
				msg := log.NewFormattedMessage(c.format, c.args)
				for j := 0; j < n; j++ {
					_ = msg.Text()
				}
			}
		}
		cost0 := time.Since(start)

		start = time.Now()
		for i := 0; i < b.N; i++ {
			for _, c := range testcases {
				msg := &FormattedMessage{
					format: c.format,
					args:   c.args,
				}
				for j := 0; j < n; j++ {
					_ = msg.Text()
				}
			}
		}
		cost1 := time.Since(start)

		start = time.Now()
		for i := 0; i < b.N; i++ {
			for _, c := range testcases {
				msg := &OptimizedFormattedMessage{
					format: c.format,
					args:   c.args,
				}
				for j := 0; j < n; j++ {
					_ = msg.Text()
				}
			}
		}
		cost2 := time.Since(start)

		if b.N == 1 {
			continue
		}
		fmt.Printf("N=%d\t n=%d\t %v\t %v\t %v\n", b.N, n, cost0, cost1, cost2)
	}
}

func TestFormattedMessage(t *testing.T) {
	testcases := []struct {
		format string
		args   []interface{}
		expect string
	}{
		{
			args:   []interface{}{"a", "%", "b"},
			expect: "a%b",
		},
		{
			args: []interface{}{
				func() []interface{} {
					return util.T("a", "%", "b")
				},
			},
			expect: "a%b",
		},
		{
			format: "%s%%%s",
			args:   []interface{}{"a", "b"},
			expect: "a%b",
		},
		{
			format: "%s%%%s",
			args: []interface{}{
				func() []interface{} {
					return util.T("a", "b")
				},
			},
			expect: "a%b",
		},
	}
	for _, c := range testcases {
		msg := log.NewFormattedMessage(c.format, c.args)
		assert.Equal(t, msg.Text(), c.expect)
	}
}

// JsonMessage can convert itself to text by json.Marshal.
type JsonMessage struct {
	data interface{}
	text atomic.Value
}

func (msg *JsonMessage) gen() string {
	b, err := json.Marshal(msg.data)
	if err != nil {
		return err.Error()
	}
	return string(b)
}

func (msg *JsonMessage) Text() string {
	v := msg.text.Load()
	if v == nil {
		text := msg.gen()
		msg.text.Store(text)
		return text
	}
	return v.(string)
}

type OptimizedJsonMessage struct {
	log.OptimizedMessage
	data interface{}
}

func (msg *OptimizedJsonMessage) Text() string {
	return msg.Once(func() string {
		b, err := json.Marshal(msg.data)
		if err != nil {
			return err.Error()
		}
		return string(b)
	})
}

func BenchmarkJsonMessage(b *testing.B) {

	testcases := []struct {
		data   interface{}
		expect string
	}{
		{
			data:   []interface{}{"a", "%", "b"},
			expect: `["a","%","b"]`,
		},
		{
			data: map[string]interface{}{
				"1": "a",
				"2": "%",
				"3": "b",
			},
			expect: `{"1":"a","2":"%","3":"b"}`,
		},
	}

	for _, n := range []int{1, 2, 3} {

		start := time.Now()
		for i := 0; i < b.N; i++ {
			for _, c := range testcases {
				msg := log.NewJsonMessage(c.data)
				for j := 0; j < n; j++ {
					_ = msg.Text()
				}
			}
		}
		cost0 := time.Since(start)

		start = time.Now()
		for i := 0; i < b.N; i++ {
			for _, c := range testcases {
				msg := &JsonMessage{
					data: c.data,
				}
				for j := 0; j < n; j++ {
					_ = msg.Text()
				}
			}
		}
		cost1 := time.Since(start)

		start = time.Now()
		for i := 0; i < b.N; i++ {
			for _, c := range testcases {
				msg := &OptimizedJsonMessage{
					data: c.data,
				}
				for j := 0; j < n; j++ {
					_ = msg.Text()
				}
			}
		}
		cost2 := time.Since(start)

		if b.N == 1 {
			continue
		}
		fmt.Printf("N=%d\t n=%d\t %v\t %v\t %v\n", b.N, n, cost0, cost1, cost2)
	}
}

func TestJsonMessage(t *testing.T) {
	testcases := []struct {
		data   interface{}
		expect string
	}{
		{
			data:   []interface{}{"a", "%", "b"},
			expect: `["a","%","b"]`,
		},
		{
			data: map[string]interface{}{
				"1": "a",
				"2": "%",
				"3": "b",
			},
			expect: `{"1":"a","2":"%","3":"b"}`,
		},
	}
	for _, c := range testcases {
		msg := log.NewJsonMessage(c.data)
		assert.Equal(t, msg.Text(), c.expect)
	}
}
