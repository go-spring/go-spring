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

package SpringCore_test

import (
	"fmt"
	"testing"

	"github.com/go-spring/go-spring/spring-core"
	"github.com/magiconair/properties/assert"
)

func TestDefaultProperties_LoadProperties(t *testing.T) {

	p := SpringCore.NewDefaultProperties()
	p.LoadProperties("testdata/config/application.yaml")

	for k, v := range p.GetAllProperties() {
		fmt.Println(k, v)
	}
}

func TestRegisterTypeConverter(t *testing.T) {

	assert.Panic(t, func() { // 不是函数
		SpringCore.RegisterTypeConverter(3)
	}, "fn must be func\\(string\\)struct")

	assert.Panic(t, func() { // 入参太多
		SpringCore.RegisterTypeConverter(func(_ string, _ string) Point {
			return Point{}
		})
	}, "fn must be func\\(string\\)struct")

	assert.Panic(t, func() { // 返回值太多
		SpringCore.RegisterTypeConverter(func(_ string) (Point, Point) {
			return Point{}, Point{}
		})
	}, "fn must be func\\(string\\)struct")

	SpringCore.RegisterTypeConverter(PointConverter)
}
