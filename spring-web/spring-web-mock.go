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

import "strings"

type MockData struct {
	Method string // 请求方法
	Path   string // 请求路径
	Data   string // 返回数据
}

type MockController struct {
	Mapping map[string]*MockData
}

func (controller *MockController) InitController(c WebContainer) {
	for _, mockData := range controller.Mapping {
		switch strings.ToLower(mockData.Method) {
		case "get":
			c.GET(mockData.Path, controller.Mock)
		case "post":
			c.POST(mockData.Path, controller.Mock)
		default:
			panic("暂不支持该方法!")
		}
	}
}

func (controller *MockController) Mock(ctx *SpringWebContext) interface{} {
	key := strings.ToLower(ctx.R.Method) + ":" + ctx.R.URL.Path
	return controller.Mapping[key].Data
}
