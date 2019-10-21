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
	"fmt"
	"testing"
)

func TestRange(t *testing.T) {

	i := 0

	f := func() []int {
		i++
		return []int{5, 6, 7, 8}
	}

	// range 用法中的 f() 只调用一次
	for _, v := range f() {
		fmt.Println(v)
	}

	fmt.Println(i)
}

func TestConfigParserViper(t *testing.T) {

	app := NewApplication("data/")
	app.loadConfigFiles()

	appName := app.AppContext.GetStringProperty("spring.application.name")
	fmt.Println("spring.application.name=" + appName)
}
