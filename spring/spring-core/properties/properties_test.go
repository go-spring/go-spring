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

package properties_test

import (
	"testing"

	"github.com/go-spring/spring-core/assert"
	"github.com/go-spring/spring-core/properties"
)

func TestRead(t *testing.T) {

	// 不支持数组
	str := "a[0].b=c"
	m, _ := properties.Read([]byte(str))
	assert.Nil(t, m["a"])
	assert.Equal(t, m["a[0].b"], "c")

	// 不支持属性引用
	str = "a=b\nc=${a}"
	m, _ = properties.Read([]byte(str))
	assert.Equal(t, m["a"], "b")
	assert.Equal(t, m["c"], "${a}")
}
