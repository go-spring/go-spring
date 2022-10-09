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

package filter

import (
	"net/http"

	"github.com/go-spring/spring-base/log"
	"github.com/go-spring/spring-core/gs"
	"github.com/go-spring/spring-core/web"
)

func init() {
	gs.Object(&StringFilter{Text: "server"}).Export((*web.Filter)(nil))
}

type StringFilter struct {
	Logger *log.Logger `logger:""`
	Text   string
}

func (f *StringFilter) Invoke(ctx web.Context, chain web.FilterChain) {
	ctxLogger := f.Logger.WithContext(ctx.Context())
	w := &StatusResponseWriter{ResponseWriter: ctx.Response().Get()}
	ctx.Response().Set(w)

	defer func() { ctxLogger.Info("after ", f.Text, " code:", w.Status()) }()
	ctxLogger.Info("before ", f.Text)
	f.Logger.Info(f.Text)

	chain.Next(ctx, web.Recursive)
}

type StatusResponseWriter struct {
	http.ResponseWriter
	statusCode int
}

func (w *StatusResponseWriter) Status() int {
	return w.statusCode
}

func (w *StatusResponseWriter) WriteHeader(statusCode int) {
	w.statusCode = statusCode
}
