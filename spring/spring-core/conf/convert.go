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

package conf

import (
	"errors"
	"reflect"
	"time"

	"github.com/go-spring/spring-core/util"
	"github.com/go-spring/spring-core/util/cast"
)

var converters = map[reflect.Type]interface{}{}

func init() {

	// time.Time 转换函数，支持非常多的日期格式，参见 cast.StringToDate()。
	Convert(func(s string) (time.Time, error) { return cast.ToTimeE(s) })

	// time.Duration 转换函数，支持 "ns", "us" (or "µs"), "ms", "s", "m", "h" 等。
	Convert(func(s string) (time.Duration, error) { return cast.ToDurationE(s) })
}

func validConverter(t reflect.Type) bool {
	return t.Kind() == reflect.Func &&
		t.NumIn() == 1 &&
		t.In(0).Kind() == reflect.String &&
		t.NumOut() == 2 &&
		util.IsValueType(t.Out(0)) &&
		util.IsErrorType(t.Out(1))
}

// Convert 注册类型转换器，转换器的函数原型为 func(string)(type,error) 。
func Convert(fn interface{}) {
	t := reflect.TypeOf(fn)
	if !validConverter(t) {
		panic(errors.New("fn must be func(string)(type,error)"))
	}
	converters[t.Out(0)] = fn
}
