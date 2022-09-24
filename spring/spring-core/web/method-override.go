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
	"net/http"
	"strings"
)

type MethodOverrideGetter func(ctx Context) string

type MethodOverrideConfig struct {
	getters []MethodOverrideGetter
}

func NewMethodOverrideConfig() *MethodOverrideConfig {
	return new(MethodOverrideConfig)
}

func (config *MethodOverrideConfig) ByHeader(key string) *MethodOverrideConfig {
	config.getters = append(config.getters, func(ctx Context) string {
		return ctx.Header(key)
	})
	return config
}

func (config *MethodOverrideConfig) ByQueryParam(name string) *MethodOverrideConfig {
	config.getters = append(config.getters, func(ctx Context) string {
		return ctx.QueryParam(name)
	})
	return config
}

func (config *MethodOverrideConfig) ByFormValue(name string) *MethodOverrideConfig {
	config.getters = append(config.getters, func(ctx Context) string {
		return ctx.FormValue(name)
	})
	return config
}

func (config *MethodOverrideConfig) get(ctx Context) string {
	for _, getter := range config.getters {
		if method := getter(ctx); method != "" {
			return method
		}
	}
	return ""
}

func NewMethodOverrideFilter(config *MethodOverrideConfig) *Prefilter {
	if len(config.getters) == 0 {
		config.ByHeader("X-HTTP-Method").
			ByHeader("X-HTTP-Method-Override").
			ByHeader("X-Method-Override").
			ByQueryParam("_method").
			ByFormValue("_method")
	}
	return FuncPrefilter(func(ctx Context, chain FilterChain) {
		req := ctx.Request()
		if strings.ToUpper(req.Method) == http.MethodPost {
			if method := config.get(ctx); method != "" {
				req.Method = strings.ToUpper(method)
			}
		}
		chain.Next(ctx, Iterative)
	})
}
