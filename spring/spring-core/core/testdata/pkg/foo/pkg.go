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

package pkg

import (
	"context"
	"fmt"
)

// golang 允许不同的路径下存在相同的包，而且允许存在相同的包。
type SamePkg struct{}

func (p *SamePkg) Package() {
	fmt.Println("github.com/go-spring/spring-core/testdata/pkg/foo/pkg.SamePkg")
}

type appContext struct {
	// 导出 fmt.Stringer 接口
	// 这种导出方式建议写在最上面
	_ fmt.Stringer `export:""`

	// 导出 context.Context 接口
	context.Context `export:""`
}

func NewAppContext() *appContext {
	return &appContext{
		Context: context.TODO(),
	}
}

func (_ *appContext) String() string {
	return ""
}
