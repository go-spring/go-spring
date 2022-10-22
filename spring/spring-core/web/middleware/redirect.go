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
	"net/http"
	"strings"

	"github.com/go-spring/spring-core/web"
)

type RedirectConfig struct {
	Code int
}

func NewRedirectConfig() RedirectConfig {
	return RedirectConfig{
		Code: http.StatusMovedPermanently,
	}
}

// redirectFilter 重定向过滤器。
type redirectFilter struct {
	code     int
	redirect func(scheme, host, uri string) (ok bool, url string)
}

func HTTPSRedirect(config RedirectConfig) web.Filter {
	return &redirectFilter{
		code: config.Code,
		redirect: func(scheme, host, uri string) (ok bool, url string) {
			if scheme != "https" {
				return true, "https://" + host + uri
			}
			return false, ""
		},
	}
}

func HTTPSWWWRedirect(config RedirectConfig) web.Filter {
	return &redirectFilter{
		code: config.Code,
		redirect: func(scheme, host, uri string) (ok bool, url string) {
			if scheme != "https" && !strings.HasPrefix(host, "www.") {
				return true, "https://www." + host + uri
			}
			return false, ""
		},
	}
}

func HTTPSNonWWWRedirect(config RedirectConfig) web.Filter {
	return &redirectFilter{
		code: config.Code,
		redirect: func(scheme, host, uri string) (ok bool, url string) {
			if scheme != "https" {
				host = strings.TrimPrefix(host, "www.")
				return true, "https://" + host + uri
			}
			return false, ""
		},
	}
}

func WWWRedirect(config RedirectConfig) web.Filter {
	return &redirectFilter{
		code: config.Code,
		redirect: func(scheme, host, uri string) (ok bool, url string) {
			if !strings.HasPrefix(host, "www.") {
				return true, scheme + "://" + host[4:] + uri
			}
			return false, ""
		},
	}
}

func NonWWWRedirect(config RedirectConfig) web.Filter {
	return &redirectFilter{
		code: config.Code,
		redirect: func(scheme, host, uri string) (ok bool, url string) {
			if strings.HasPrefix(host, "www.") {
				return true, scheme + "://" + host[4:] + uri
			}
			return false, ""
		},
	}
}

func (f *redirectFilter) Invoke(ctx web.Context, chain web.FilterChain) {
	req := ctx.Request()
	ok, url := f.redirect(ctx.Scheme(), req.Host, req.RequestURI)
	if ok {
		code := http.StatusMovedPermanently
		if f.code != 0 {
			code = f.code
		}
		ctx.Redirect(code, url)
		return
	}
	chain.Next(ctx, web.Iterative)
}
