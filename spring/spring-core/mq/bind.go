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

	"github.com/go-spring/spring-core/json"
	"github.com/go-spring/spring-core/util"
)

// contextType context.Context 的反射类型。
var contextType = reflect.TypeOf((*context.Context)(nil)).Elem()

// BindConsumer BIND 方式实现的消息消费者。
type BindConsumer struct {
	topics   []string
	fn       interface{}
	fnType   reflect.Type
	fnValue  reflect.Value
	bindType reflect.Type
}

func (c *BindConsumer) Topics() []string { return c.topics }

func (c *BindConsumer) Consume(ctx context.Context, msg *Message) {
	bindVal := reflect.New(c.bindType.Elem())
	err := json.Unmarshal(msg.Body, bindVal.Interface())
	util.Panic(err).When(err != nil)
	c.fnValue.Call([]reflect.Value{reflect.ValueOf(ctx), bindVal})
}

func validBindFn(fnType reflect.Type) bool {

	// 必须是函数，必须有两个入参
	if fnType.Kind() != reflect.Func || fnType.NumIn() != 2 {
		return false
	}

	// 第一个入参必须是 context.Context 类型
	if fnType.In(0) != contextType {
		return false
	}

	req := fnType.In(1) // 第二个入参必须是结构体指针
	return req.Kind() == reflect.Ptr && req.Elem().Kind() == reflect.Struct
}

// BIND 创建 BIND 方式实现的消费者
func BIND(topic string, fn interface{}) *BindConsumer {
	if fnType := reflect.TypeOf(fn); validBindFn(fnType) {
		return &BindConsumer{
			topics:   []string{topic},
			fn:       fn,
			fnType:   fnType,
			fnValue:  reflect.ValueOf(fn),
			bindType: fnType.In(1),
		}
	}
	panic(errors.New("fn should be func(context.Context, *struct})"))
}
