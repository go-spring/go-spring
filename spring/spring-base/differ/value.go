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

package differ

// Strategy 比较策略。
type Strategy int

// Comparator 值比较器。
type Comparator func(a, b interface{}) bool

type DiffItem struct {
	A interface{}
	B interface{}
}

type DiffResult struct {
	Differs map[string]DiffItem
	Ignores map[string]DiffItem
	Equals  map[string]DiffItem
}

// ValueDiffer 值比较器。
type ValueDiffer struct {
}

// NewValueDiffer 创建新的值比较器。
func NewValueDiffer() *ValueDiffer {
	return &ValueDiffer{}
}

// Diff 比较 a,b 两个任意值，返回它们异同之处。
func (d *ValueDiffer) Diff(a, b interface{}) *DiffResult {
	return nil
}

// DiffValue 比较 a,b 两个任意值，返回它们异同之处。
func DiffValue(a, b interface{}) *DiffResult {
	return NewValueDiffer().Diff(a, b)
}
