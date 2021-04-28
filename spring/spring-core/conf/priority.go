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
	"sort"
)

type Priority []Properties

func (p Priority) Keys() []string {
	var oldKeys []string
	for _, c := range p {
		n := len(oldKeys)
		var newKeys []string
		for k := range c.Map() {
			i := sort.SearchStrings(oldKeys, k)
			if i < 0 || i >= n {
				newKeys = append(newKeys, k)
			}
		}
		oldKeys = append(oldKeys, newKeys...)
		sort.Strings(oldKeys)
	}
	return oldKeys
}

func (p Priority) Get(key string) interface{} {
	for _, c := range p {
		if v := c.Get(key); v != nil {
			return v
		}
	}
	return nil
}

func (p Priority) GetFirst(keys ...string) interface{} {
	for _, c := range p {
		for _, k := range keys {
			if v := c.Get(k); v != nil {
				return v
			}
		}
	}
	return nil
}
