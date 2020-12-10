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

import (
	"context"
	"sync"

	"github.com/go-spring/spring-utils"
)

// WebServer 一个 WebServer 包含多个 WebContainer
type WebServer struct {
	containers []WebContainer // Web 容器列表
	filters    []Filter       // 共用的普通过滤器
	logger     Filter         // 共用的日志过滤器
	errors     []error        // 错误列表
	errLock    sync.Mutex
}

// NewWebServer WebServer 的构造函数
func NewWebServer() *WebServer { return &WebServer{} }

// Errors 获取错误列表
func (s *WebServer) Errors() []error { return s.errors }

// Filters 获取过滤器列表
func (s *WebServer) Filters() []Filter { return s.filters }

// AddFilter 添加共用的普通过滤器
func (s *WebServer) AddFilter(filter ...Filter) *WebServer {
	s.filters = append(s.filters, filter...)
	return s
}

// SetFilters 设置过滤器列表
func (s *WebServer) SetFilters(filters []Filter) { s.filters = filters }

// GetLoggerFilter 获取 Logger Filter
func (s *WebServer) GetLoggerFilter() Filter { return s.logger }

// SetLoggerFilter 设置共用的日志过滤器
func (s *WebServer) SetLoggerFilter(filter Filter) *WebServer {
	s.logger = filter
	return s
}

// Containers 返回 WebContainer 实例列表
func (s *WebServer) Containers() []WebContainer { return s.containers }

// AddContainer 添加 WebContainer 实例
func (s *WebServer) AddContainer(container ...WebContainer) *WebServer {
	s.containers = append(s.containers, container...)
	return s
}

func (s *WebServer) error(err error) {
	if err != nil {
		s.errLock.Lock()
		defer s.errLock.Unlock()
		s.errors = append(s.errors, err)
	}
}

// Start 启动 Web 容器，非阻塞调用
func (s *WebServer) Start() {
	s.errors = make([]error, 0)
	for _, container := range s.containers {
		c := container // 避免闭包延迟捕获

		// 如果 Container 使用的是默认值的话，Container 使用 Server 的日志过滤器
		if s.logger != nil && c.GetLoggerFilter() == defaultLoggerFilter {
			c.SetLoggerFilter(s.logger)
		}

		// 添加 Server 的普通过滤器给 Container
		c.SetFilters(append(s.filters, c.GetFilters()...))

		go func() { s.error(c.Start()) }()
	}
}

// Stop 停止 Web 容器，阻塞调用
func (s *WebServer) Stop(ctx context.Context) {
	s.errors = make([]error, 0)
	var wg SpringUtils.WaitGroup
	for _, container := range s.containers {
		c := container // 避免延迟绑定
		wg.Add(func() { s.error(c.Stop(ctx)) })
	}
	wg.Wait()
}
