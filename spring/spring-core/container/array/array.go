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

package array

// Array 数组
type Array struct{ data []interface{} }

// New Array 的构造函数。
func New() *Array {
	return &Array{data: make([]interface{}, 0)}
}

// NewSize Array 的构造函数。
func NewSize(size int) *Array {
	return &Array{data: make([]interface{}, size)}
}

// Len 返回 Array 的长度。
func (arr *Array) Len() int {
	return len(arr.data)
}

// Append 向数组尾部添加一个元素。
func (arr *Array) Append(i interface{}) {
	arr.data = append(arr.data, i)
}

// Get 获取 idx 位置的元素。
func (arr *Array) Get(idx int) interface{} {
	return arr.data[idx]
}
