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

// Package json 提供封装其他 json 序列化框架的接口。
package json

import (
	"bytes"
	"encoding/json"
	"errors"
)

// MarshalFunc 定义 Marshal 函数原型。
type MarshalFunc func(v interface{}) ([]byte, error)

// UnmarshalFunc 定义 Unmarshal 函数原型。
type UnmarshalFunc func(data []byte, v interface{}) error

var (
	marshal   MarshalFunc
	unmarshal UnmarshalFunc
)

// Init 自定义 Marshal 和 Unmarshal 函数，为了防止在一个项目中存在多个
// json 序列化框架，该函数只能在项目 main 函数执行前的 init 阶段设置一次。
func Init(m MarshalFunc, u UnmarshalFunc) {
	if m == nil || u == nil {
		panic(errors.New("invalid param"))
	}
	if marshal != nil {
		panic(errors.New("marshal not nil"))
	}
	marshal = m
	unmarshal = u
}

// ToString 将对象序列化为 Json 字符串，错误信息以字符串形式返回。
func ToString(i interface{}) string {
	b, err := Marshal(i)
	if err != nil {
		return err.Error()
	}
	return string(b)
}

// Marshal 序列化 json 数据。
func Marshal(v interface{}) ([]byte, error) {
	if marshal != nil {
		return marshal(v)
	}
	return json.Marshal(v)
}

// MarshalIndent 序列化 json 数据并对结果进行美化。
func MarshalIndent(v interface{}, prefix, indent string) ([]byte, error) {
	b, err := Marshal(v)
	if err != nil {
		return nil, err
	}
	var buf bytes.Buffer
	err = json.Indent(&buf, b, prefix, indent)
	if err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// Unmarshal 反序列化 json 对象。
func Unmarshal(data []byte, v interface{}) error {
	if unmarshal != nil {
		return unmarshal(data, v)
	}
	d := json.NewDecoder(bytes.NewReader(data))
	//d.UseNumber()
	return d.Decode(v)
}
