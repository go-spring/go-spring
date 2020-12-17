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

	"github.com/go-spring/spring-utils"
)

func TestRouter_Route(t *testing.T) {
	root := Route("/root", FilterBean("r1", "r2")).ConditionOnBean("r")

	get := root.GetMapping("/get", nil, FilterBean("g1", "g2")).ConditionOnBean("g")
	SpringUtils.AssertEqual(t, get, DefaultWebMapping.Mappings[get.Key()])
	SpringUtils.AssertEqual(t, get.Path(), "/root/get")
	SpringUtils.AssertEqual(t, len(get.Filters()), 2)
	// TODO 校验 cond 字段是否正确

	sub := root.Route("/sub", FilterBean("s1", "s2")).ConditionOnBean("s")
	subGet := sub.GetMapping("/get", nil, FilterBean("sg1", "sg2")).ConditionOnBean("sg")
	SpringUtils.AssertEqual(t, subGet, DefaultWebMapping.Mappings[subGet.Key()])
	SpringUtils.AssertEqual(t, subGet.Path(), "/root/sub/get")
	SpringUtils.AssertEqual(t, len(subGet.Filters()), 3)
	// ...

	subSub := sub.Route("/sub", FilterBean("ss1", "ss2")).ConditionOnBean("ss")
	subSubGet := subSub.GetMapping("/get", nil, FilterBean("ssg1", "ssg2")).ConditionOnBean("ssg")
	SpringUtils.AssertEqual(t, subSubGet, DefaultWebMapping.Mappings[subSubGet.Key()])
	SpringUtils.AssertEqual(t, subSubGet.Path(), "/root/sub/sub/get")
	SpringUtils.AssertEqual(t, len(subSubGet.Filters()), 4)
	// ...
}
