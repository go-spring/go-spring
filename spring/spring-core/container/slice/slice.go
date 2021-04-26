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

package slice

// empty 创建一个空的切片。
var empty = make([]interface{}, 0)

// Slice 封装 golang 内置的切片，更方便使用。
type Slice struct{ data []interface{} }

// New 创建空的切片。
func New() *Slice {
	return &Slice{data: empty}
}

// NewSize 创建 size 大小的切片。
func NewSize(size int) *Slice {
	return &Slice{data: make([]interface{}, size)}
}

// Len 返回切片的长度。
func (arr *Slice) Len() int {
	return len(arr.data)
}

// Append 向切片尾部添加一个元素。
func (arr *Slice) Append(i interface{}) {
	arr.data = append(arr.data, i)
}

// Get 获取 i 位置的元素，需要用户保证 i 不越界。
func (arr *Slice) Get(i int) interface{} {
	return arr.data[i]
}
