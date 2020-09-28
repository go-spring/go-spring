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

package SpringBoot

import (
	"testing"

	"github.com/magiconair/properties/assert"
)

func TestRouter_Route(t *testing.T) {
	root := Route("/root", FilterBean("r1", "r2")).ConditionOnBean("r").OnPorts(9090)

	get := root.GetMapping("/get", nil, FilterBean("g1", "g2")).ConditionOnBean("g").OnPorts(8080)
	assert.Equal(t, get, DefaultWebMapping.Mappings[get.Key()])
	assert.Equal(t, get.Ports(), []int{9090, 8080})
	assert.Equal(t, get.Path(), "/root/get")
	assert.Equal(t, len(get.Filters()), 2)
	// TODO 校验 cond 字段是否正确

	sub := root.Route("/sub", FilterBean("s1", "s2")).ConditionOnBean("s").OnPorts(7070)
	subGet := sub.GetMapping("/get", nil, FilterBean("sg1", "sg2")).ConditionOnBean("sg").OnPorts(6060)
	assert.Equal(t, subGet, DefaultWebMapping.Mappings[subGet.Key()])
	assert.Equal(t, subGet.Ports(), []int{9090, 7070, 6060})
	assert.Equal(t, subGet.Path(), "/root/sub/get")
	assert.Equal(t, len(subGet.Filters()), 3)
	// ...

	subSub := sub.Route("/sub", FilterBean("ss1", "ss2")).ConditionOnBean("ss").OnPorts(5050)
	subSubGet := subSub.GetMapping("/get", nil, FilterBean("ssg1", "ssg2")).ConditionOnBean("ssg").OnPorts(4040)
	assert.Equal(t, subSubGet, DefaultWebMapping.Mappings[subSubGet.Key()])
	assert.Equal(t, subSubGet.Ports(), []int{9090, 7070, 5050, 4040})
	assert.Equal(t, subSubGet.Path(), "/root/sub/sub/get")
	assert.Equal(t, len(subSubGet.Filters()), 4)
	// ...
}
