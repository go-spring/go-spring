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
	"strings"
	"time"

	"github.com/go-spring/spring-base/cast"
	"github.com/go-spring/spring-base/util"
)

// Converter 类型转换器，函数原型为 func(string)(type,error)。
type Converter interface{}

// Splitter 字符串分割器，用于将字符串按逗号分割成字符串切片。
type Splitter func(string) ([]string, error)

var (
	splitters  = map[string]Splitter{}
	converters = map[reflect.Type]Converter{}
)

func validConverter(t reflect.Type) bool {
	return t.Kind() == reflect.Func &&
		t.NumIn() == 1 &&
		t.In(0).Kind() == reflect.String &&
		t.NumOut() == 2 &&
		IsValueType(t.Out(0)) &&
		util.IsErrorType(t.Out(1))
}

// RegisterConverter 注册类型转换器。
func RegisterConverter(fn interface{}) {
	t := reflect.TypeOf(fn)
	if !validConverter(t) {
		panic(errors.New("fn must be func(string)(type,error)"))
	}
	converters[t.Out(0)] = fn
}

// RegisterSplitter 注册字符串分割器。
func RegisterSplitter(name string, fn Splitter) {
	splitters[name] = fn
}

// TimeConverter 转换函数，支持时间戳格式，支持日期字符串(日期字符串>>日期字符串的格式)。
func TimeConverter(s string) (time.Time, error) {
	format := "2006-01-02 15:04:05 -0700"
	if ss := strings.Split(s, ">>"); len(ss) == 2 {
		format = strings.TrimSpace(ss[1])
		s = strings.TrimSpace(ss[0])
	}
	return cast.ToTimeE(s, format)
}

// DurationConverter 转换函数，支持 "ns", "us" (or "µs"), "ms", "s", "m", "h" 等。
func DurationConverter(s string) (time.Duration, error) {
	return cast.ToDurationE(s)
}
