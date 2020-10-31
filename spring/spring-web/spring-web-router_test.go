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

package SpringWeb

import (
	"testing"

	"github.com/magiconair/properties/assert"
)

func TestRouter_Route(t *testing.T) {
	// TODO 将第二个 loggerFilter 改为其他类型的 Filter
	root := NewRouter("/root", &loggerFilter{}, &loggerFilter{})

	get := root.GetMapping("/get", nil, &loggerFilter{}, &loggerFilter{})
	assert.Equal(t, get.path, "/root/get")
	assert.Equal(t, len(get.filters), 4)

	sub := root.Route("/sub", &loggerFilter{}, &loggerFilter{})
	subGet := sub.GetMapping("/get", nil, &loggerFilter{}, &loggerFilter{})
	assert.Equal(t, subGet.path, "/root/sub/get")
	assert.Equal(t, len(subGet.filters), 6)

	subSub := sub.Route("/sub", &loggerFilter{}, &loggerFilter{})
	subSubGet := subSub.GetMapping("/get", nil, &loggerFilter{}, &loggerFilter{})
	assert.Equal(t, subSubGet.path, "/root/sub/sub/get")
	assert.Equal(t, len(subSubGet.filters), 8)
}
