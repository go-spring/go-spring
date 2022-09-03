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
	"compress/gzip"
	"github.com/go-spring/spring-core/web"
	"net/http"
	"path/filepath"
	"strings"
)

// compress writer by gzip

func GzipResponseFilter(level int) web.Filter {
	return web.FuncFilter(func(ctx web.Context, chain web.FilterChain) {
		if level == 0 {
			chain.Next(ctx)
			return
		}

		if !shouldCompress(ctx.Request()) {
			chain.Next(ctx)
			return
		}

		gw, err := gzip.NewWriterLevel(ctx.Response().Get(), level)
		if err != nil {
			chain.Next(ctx)
			return
		}

		ctx.SetHeader(web.HeaderContentEncoding, "gzip")
		ctx.SetHeader(web.HeaderVary, web.HeaderAcceptEncoding)
		defer func() {
			ctx.SetHeader(web.HeaderContentLength, "0")
			_ = gw.Close()
		}()

		ctx.Response().Set(&gzipWriter{ctx.Response().Get(), gw})
		chain.Next(ctx)
	})
}

func shouldCompress(req *http.Request) bool {
	if !strings.Contains(req.Header.Get(web.HeaderAcceptEncoding), "gzip") {
		return false
	}

	extension := filepath.Ext(req.URL.Path)
	if len(extension) < 4 {
		return true
	}

	switch extension {
	case ".png", ".gif", ".jpeg", ".jpg":
		return false
	default:
		return true
	}
}

type gzipWriter struct {
	http.ResponseWriter
	gw *gzip.Writer
}

func (g *gzipWriter) Write(data []byte) (int, error) {
	return g.gw.Write(data)
}
