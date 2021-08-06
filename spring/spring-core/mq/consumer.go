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

package mq

import (
	"context"
	"errors"
	"reflect"

	"github.com/go-spring/spring-boost/json"
	"github.com/go-spring/spring-boost/util"
)

// Consumer 消息消费者。
type Consumer interface {
	Topics() []string
	Consume(ctx context.Context, msg Message) error
}

// consumer Bind 方式的消息消费者。
type consumer struct {

	// 消息主题列表。
	topics []string

	fn interface{}
	t  reflect.Type
	v  reflect.Value
	e  reflect.Type
}

func (c *consumer) Topics() []string {
	return c.topics
}

func (c *consumer) Consume(ctx context.Context, msg Message) error {
	e := reflect.New(c.e.Elem())
	err := json.Unmarshal(msg.Body(), e.Interface())
	if err != nil {
		return err
	}
	out := c.v.Call([]reflect.Value{reflect.ValueOf(ctx), e})
	if err = out[0].Interface().(error); err != nil {
		return err
	}
	return nil
}

func validBindFn(t reflect.Type) bool {
	return util.IsFuncType(t) &&
		util.ReturnOnlyError(t) &&
		t.NumIn() == 2 &&
		util.IsContextType(t.In(0)) &&
		util.IsStructPtr(t.In(1))
}

// Bind 创建 Bind 方式的消费者。
func Bind(fn interface{}, topics ...string) *consumer {
	if t := reflect.TypeOf(fn); validBindFn(t) {
		return &consumer{
			topics: topics,
			fn:     fn,
			t:      t,
			v:      reflect.ValueOf(fn),
			e:      t.In(1),
		}
	}
	panic(errors.New("fn should be func(ctx,*struct)error"))
}
