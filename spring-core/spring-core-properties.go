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

import (
	"strings"
)

func (ctx *DefaultSpringContext) GetProperties(name string) string {
	return ctx.properties[name]
}

func (ctx *DefaultSpringContext) SetProperties(name string, value string) {
	ctx.properties[name] = value
}

func (ctx *DefaultSpringContext) GetPrefixProperties(prefix string) map[string]string {
	result := make(map[string]string)
	for k, v := range ctx.properties {
		if strings.HasPrefix(k, prefix) {
			result[k] = v
		}
	}
	return result
}

func (ctx *DefaultSpringContext) GetDefaultProperties(name string, defaultValue string) (string, bool) {
	if v, ok := ctx.properties[name]; ok {
		return v, true
	}
	return defaultValue, false
}
