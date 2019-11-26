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

package SpringCore

//
// 定义 tag 列表接口
//
type TagList interface {
	Get(i int) (string, bool)
}

//
// 基于数组的 TagList 实现
//
type ArrayTagList struct {
	tags []string
}

//
// 工厂函数
//
func NewArrayTagList(tags ...string) *ArrayTagList {
	return &ArrayTagList{
		tags: tags,
	}
}

func (t *ArrayTagList) Get(i int) (string, bool) {
	if i >= len(t.tags) {
		return "", false
	}
	return t.tags[i], true
}

//
// 基于下标的 TagList 实现
//
type IndexTagList struct {
	tags map[int]string
}

//
// 工厂函数
//
func NewIndexTagList(tags map[int]string) *IndexTagList {
	return &IndexTagList{
		tags: tags,
	}
}

func (t *IndexTagList) Get(i int) (string, bool) {
	tag, ok := t.tags[i]
	return tag, ok
}
