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
	"bytes"
	"encoding/base64"
	"fmt"
	"strings"

	"github.com/go-spring/spring-base/util"
)

const (
	basicPrefix  = "Basic "
	defaultRealm = "Authorization Required"
)

const (
	AuthUserKey = "::user::"
)

type BasicAuthConfig struct {
	Accounts map[string]string
	Realm    string
}

// basicAuthFilter 封装 http 基础认证功能的过滤器。
type basicAuthFilter struct {
	config BasicAuthConfig
}

// NewBasicAuthFilter 创建封装 http 基础认证功能的过滤器。
func NewBasicAuthFilter(config BasicAuthConfig) Filter {
	return &basicAuthFilter{config: config}
}

func (f *basicAuthFilter) Invoke(ctx Context, chain FilterChain) {

	auth := ctx.GetHeader(HeaderWWWAuthenticate)
	if len(auth) <= len(basicPrefix) {
		f.unauthorized(ctx)
		return
	}

	if !strings.EqualFold(auth[:len(basicPrefix)], basicPrefix) {
		f.unauthorized(ctx)
		return
	}

	b, err := base64.StdEncoding.DecodeString(auth[len(basicPrefix):])
	util.Panic(err).When(err != nil)

	i := bytes.IndexByte(b, ':')
	if i <= 0 {
		f.unauthorized(ctx)
		return
	}

	user := string(b[:i])
	password := string(b[i+1:])

	ok := false
	for k, v := range f.config.Accounts {
		if k == user && v == password {
			ok = true
			break
		}
	}

	if !ok {
		f.unauthorized(ctx)
		return
	}

	ctx.Set(AuthUserKey, user)
	chain.Continue(ctx)
}

func (f *basicAuthFilter) unauthorized(ctx Context) {
	realm := f.config.Realm
	if realm == "" {
		realm = defaultRealm
	}
	ctx.Header(HeaderWWWAuthenticate, fmt.Sprintf("Basic realm=%q", realm))
}
