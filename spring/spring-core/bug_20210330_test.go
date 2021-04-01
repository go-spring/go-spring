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
	"testing"
	"time"

	"github.com/go-spring/spring-core"
	"github.com/magiconair/properties/assert"
)

type i20210330 interface {
	Date() time.Time
}

type s20210330 struct {
	_ i20210330 `export:""`
}

func (s *s20210330) Date() time.Time {
	return time.Now()
}

func Test20210330(t *testing.T) {
	count := 0
	for i := 0; i < 20; i++ {

		// 如果 1 在 2 之前初始化则必须保证 2 导出所有接口
		ctx := SpringCore.NewDefaultSpringContext()
		ctx.RegisterNameBean("1", new(int)).ConditionOnBean((*i20210330)(nil))
		ctx.RegisterNameBean("2", new(s20210330))
		ctx.AutoWireBeans()

		var i *int
		if !ctx.GetBean(&i) {
			count++
		}
	}
	assert.Equal(t, count == 0, true)
}
