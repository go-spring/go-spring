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

package middleware

import (
	"github.com/go-spring/spring-core/web"
	"github.com/google/uuid"
)

type RequestIDConfig struct {
	Header    string
	Generator func() string
}

func NewRequestIDConfig() RequestIDConfig {
	return RequestIDConfig{}
}

func NewRequestIDFilter(config RequestIDConfig) web.Filter {
	if config.Header == "" {
		config.Header = web.HeaderXRequestID
	}
	if config.Generator == nil {
		config.Generator = func() string {
			return uuid.New().String()
		}
	}
	return web.FuncFilter(func(ctx web.Context, chain web.FilterChain) {
		reqID := ctx.Header(config.Header)
		if reqID == "" {
			reqID = config.Generator()
		}
		ctx.SetHeader(web.HeaderXRequestID, reqID)
		chain.Next(ctx, web.Iterative)
	})
}
