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

//go:generate mockgen -build_flags="-mod=mod" -package=jpath -source=jpath.go -destination=jpath_mock.go

package jpath

type Path interface {
	Read(val interface{}) map[string]interface{}
}

type JsonPath struct {
	expr string
}

// Compile 预编译 JSON 路径表达式。
func Compile(expr string) Path {
	return &JsonPath{expr: expr}
}

// Read 读取指定路径上的值，并返回这些值的集合。
func (p *JsonPath) Read(val interface{}) map[string]interface{} {
	return nil
}

// Read 读取指定路径上的值，并返回这些值的集合。
func Read(val interface{}, paths ...Path) map[string]interface{} {
	ret := make(map[string]interface{})
	for _, p := range paths {
		r := p.Read(val)
		for k, v := range r {
			ret[k] = v
		}
	}
	return ret
}
