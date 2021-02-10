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

package web

// Swagger 与容器绑定的 API 描述文档
type Swagger interface {

	// ReadDoc 读取标准格式的描述文档
	ReadDoc() string

	// AddPath 添加与容器绑定的路由节点
	AddPath(path string, method uint32, op Operation)
}

// Operation 与路由绑定的 API 描述文档
type Operation interface {

	// Process 完成处理参数绑定等过程
	Process() error
}
