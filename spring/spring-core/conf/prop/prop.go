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

import "github.com/magiconair/properties"

// Read 从内存中读取配置项列表，b 是 UTF8 格式。
func Read(b []byte) (map[string]interface{}, error) {

	p := properties.NewProperties()
	p.DisableExpansion = true

	err := p.Load(b, properties.UTF8)
	if err != nil {
		return nil, err
	}

	ret := make(map[string]interface{})
	for k, v := range p.Map() {
		ret[k] = v
	}
	return ret, nil
}
