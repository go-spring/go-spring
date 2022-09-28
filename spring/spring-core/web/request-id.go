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

import (
	"github.com/google/uuid"
)

type RequestIDConfig struct {
	Header    string
	Generator func() string
}

func NewRequestIDConfig() RequestIDConfig {
	return RequestIDConfig{}
}

func NewRequestIDFilter(config RequestIDConfig) Filter {
	if config.Header == "" {
		config.Header = HeaderXRequestID
	}
	if config.Generator == nil {
		config.Generator = func() string {
			return uuid.New().String()
		}
	}
	return FuncFilter(func(ctx Context, chain FilterChain) {
		reqID := ctx.Header(config.Header)
		if reqID == "" {
			reqID = config.Generator()
		}
		ctx.SetHeader(HeaderXRequestID, reqID)
		chain.Next(ctx, Iterative)
	})
}
