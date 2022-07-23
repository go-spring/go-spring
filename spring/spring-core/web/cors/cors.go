/*
 * Copyright 2012-2022 the original author or authors.
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

package cors

import (
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/go-spring/spring-core/web"
)

type CorsFilter struct {
	CorsConfig

	allowedOrigins   []string
	allowedMethods   []string
	allowedHeaders   []string
	exposedHeaders   []string
	allowCredentials bool
	maxAge           time.Duration

	allowAllOrigins bool
	allowAllMethods bool
	allowAllHeaders bool
}

func New(config CorsConfig) web.Filter {
	filter := &CorsFilter{}
	filter.CorsConfig = config
	filter.setConfig()

	return filter
}

func (f *CorsFilter) setConfig() {
	f.allowedOrigins = f.CorsConfig.AllowOrigins
	f.allowedHeaders = f.CorsConfig.AllowHeaders
	f.allowedMethods = f.CorsConfig.AllowMethods
	f.allowCredentials = f.CorsConfig.AllowCredentials
	f.exposedHeaders = f.CorsConfig.ExposeHeaders
	f.maxAge = f.CorsConfig.MaxAge

	f.normalizeConfig()
}

func (f *CorsFilter) normalizeConfig() {
	f.allowedHeaders = arraymap(f.allowedHeaders, strings.ToLower)
	f.allowedMethods = arraymap(f.allowedMethods, strings.ToUpper)

	f.allowAllOrigins = inarray(f.allowedOrigins, "*")
	f.allowAllHeaders = inarray(f.allowedHeaders, "*")
	f.allowAllMethods = inarray(f.allowedMethods, "*")
}

func (f *CorsFilter) isPreflightRequest(ctx web.Context) bool {
	return ctx.Request().Method == http.MethodOptions
}

func (f *CorsFilter) isCorsRequest(ctx web.Context) bool {
	return ctx.Header("Origin") != ""
}

func (f *CorsFilter) isOriginAllowed(ctx web.Context) bool {
	if f.allowAllOrigins {
		return true
	}

	origin := ctx.Header("Origin")
	if inarray(f.allowedOrigins, origin) {
		return true
	}

	return false
}

func (f *CorsFilter) configureAllowedOrigin(ctx web.Context) {
	if f.allowAllOrigins && !f.allowCredentials {
		ctx.SetHeader("Access-Control-Allow-Origin", "*")
	} else if f.isSingleOriginAllowed() {
		ctx.SetHeader("Access-Control-Allow-Origin", f.allowedOrigins[0])
	} else {
		if f.isCorsRequest(ctx) && f.isOriginAllowed(ctx) {
			ctx.SetHeader("Access-Control-Allow-Origin", ctx.Header("Origin"))
		}
		f.varyHeader(ctx, "Origin")
	}
}

func (f *CorsFilter) isSingleOriginAllowed() bool {
	if f.allowAllOrigins {
		return false
	}
	return len(f.allowedOrigins) == 1
}

func (f *CorsFilter) configureAllowedMethods(ctx web.Context) {
	var allowMehods string
	if f.allowAllMethods {
		allowMehods = strings.ToUpper(ctx.Header("Access-Control-Request-Method"))
		f.varyHeader(ctx, "Access-Control-Request-Method")
	} else {
		allowMehods = strings.Join(f.allowedMethods, ",")
	}

	ctx.SetHeader("Access-Control-Allow-Methods", allowMehods)
}

func (f *CorsFilter) configureAllowedHeaders(ctx web.Context) {
	var allowHeaders string
	if f.allowAllHeaders {
		allowHeaders = strings.ToUpper(ctx.Header("Access-Control-Request-Headers"))
		f.varyHeader(ctx, "Access-Control-Request-Headers")
	} else {
		allowHeaders = strings.Join(f.allowedHeaders, ",")
	}

	ctx.SetHeader("Access-Control-Allow-Headers", allowHeaders)
}

func (f *CorsFilter) configureAllowCredentials(ctx web.Context) {
	if f.allowCredentials {
		ctx.SetHeader("Access-Control-Allow-Credentials", "true")
	}
}

func (f *CorsFilter) configureExposedHeaders(ctx web.Context) {
	if len(f.exposedHeaders) > 0 {
		ctx.SetHeader("Access-Control-Expose-Headers", strings.Join(f.exposedHeaders, ","))
	}
}

func (f *CorsFilter) configureMaxAge(ctx web.Context) {
	if f.maxAge > 0 {
		ctx.SetHeader("Access-Control-Max-Age", strconv.FormatInt(int64(f.maxAge/time.Second), 10))
	}
}

func (f *CorsFilter) addActualRequestHeaders(ctx web.Context) {
	f.configureAllowedOrigin(ctx)
	f.configureAllowCredentials(ctx)
	f.configureExposedHeaders(ctx)
}

func (f *CorsFilter) handlePreflightRequest(ctx web.Context) {
	f.addPreflightRequestHeaders(ctx)
	ctx.SetStatus(http.StatusNoContent)
}

func (f *CorsFilter) addPreflightRequestHeaders(ctx web.Context) {
	f.configureAllowedOrigin(ctx)
	f.configureAllowCredentials(ctx)
	f.configureAllowedMethods(ctx)
	f.configureAllowedHeaders(ctx)
	f.configureMaxAge(ctx)
}

func (f *CorsFilter) varyHeader(ctx web.Context, header string) {
	varys := strings.Split(ctx.Header("Vary"), ",")
	if inarray(varys, header) {
		return
	}

	varys = append(varys, header)
	ctx.SetHeader("Vary", strings.Join(varys, ","))
}

func (f *CorsFilter) Invoke(ctx web.Context, chain web.FilterChain) {
	if f.isPreflightRequest(ctx) {
		f.handlePreflightRequest(ctx)
		f.varyHeader(ctx, "Access-Control-Request-Method")
		return
	}

	if ctx.Request().Method == http.MethodOptions {
		f.varyHeader(ctx, "Access-Control-Request-Method")
	}
	f.addActualRequestHeaders(ctx)
	chain.Next(ctx)
}
