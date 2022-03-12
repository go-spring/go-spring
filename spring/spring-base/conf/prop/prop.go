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

package prop

import (
	"fmt"
	"sort"
	"strings"

	"github.com/magiconair/properties"
)

// Read 将 properties 格式的字节数组解析成 map 数据。
func Read(b []byte) (map[string]interface{}, error) {

	p := properties.NewProperties()
	p.DisableExpansion = true

	err := p.Load(b, properties.UTF8)
	if err != nil {
		return nil, err
	}

	m := p.Map()
	var keys []string
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	ret := make(map[string]interface{})
	for _, k := range keys {
		v := m[k]
		if k[len(k)-1] == ']' {
			i := strings.LastIndex(k, "[")
			if i <= 0 {
				return nil, fmt.Errorf("invalid key %q", k)
			}
			k = k[0:i]
			if s, ok := ret[k]; ok {
				v = s.(string) + "," + v
			}
		}
		ret[k] = v
	}
	return ret, nil
}
