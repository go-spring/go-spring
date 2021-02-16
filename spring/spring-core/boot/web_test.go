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

package boot_test

import (
	"fmt"
	"testing"

	"github.com/go-spring/spring-core/boot"
	"github.com/go-spring/spring-core/web"
)

func TestMapper(t *testing.T) {
	boot.MappingGet("/", func(ctx web.Context) {})
	for _, mapper := range boot.WebMapping {
		fmt.Println(mapper.Key(), web.GetMethod(mapper.Method()))
	}
}

//func TestRouter_Route(t *testing.T) {
//	root := Route("/root", FilterBean("r1", "r2")).WithCondition(cond.OnBean("r"))
//
//	get := root.GetMapping("/get", nil, FilterBean("g1", "g2")).WithCondition(cond.OnBean("g"))
//	util.AssertEqual(t, get, DefaultWebMapping.Mappings[get.Key()])
//	util.AssertEqual(t, get.Path(), "/root/get")
//	util.AssertEqual(t, len(get.Filters()), 2)
//	// TODO 校验 cond 字段是否正确
//
//	sub := root.Route("/sub", FilterBean("s1", "s2")).WithCondition(cond.OnBean("s"))
//	subGet := sub.GetMapping("/get", nil, FilterBean("sg1", "sg2")).WithCondition(cond.OnBean("sg"))
//	util.AssertEqual(t, subGet, DefaultWebMapping.Mappings[subGet.Key()])
//	util.AssertEqual(t, subGet.Path(), "/root/sub/get")
//	util.AssertEqual(t, len(subGet.Filters()), 3)
//	// ...
//
//	subSub := sub.Route("/sub", FilterBean("ss1", "ss2")).WithCondition(cond.OnBean("ss"))
//	subSubGet := subSub.GetMapping("/get", nil, FilterBean("ssg1", "ssg2")).WithCondition(cond.OnBean("ssg"))
//	util.AssertEqual(t, subSubGet, DefaultWebMapping.Mappings[subSubGet.Key()])
//	util.AssertEqual(t, subSubGet.Path(), "/root/sub/sub/get")
//	util.AssertEqual(t, len(subSubGet.Filters()), 4)
//	// ...
//}
