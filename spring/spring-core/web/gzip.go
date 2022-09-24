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
	"compress/gzip"
	"io/ioutil"
	"net/http"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"
)

type GzipFilter struct {
	pool                 *sync.Pool
	ExcludedExtensions   []string
	ExcludedPaths        []string
	ExcludedPathsRegexes []*regexp.Regexp
}

// NewGzipFilter The compression level can be gzip.DefaultCompression,
// gzip.NoCompression, gzip.HuffmanOnly or any integer value between
// gip.BestSpeed and gzip.BestCompression inclusive.
func NewGzipFilter(level int) (Filter, error) {
	_, err := gzip.NewWriterLevel(ioutil.Discard, level)
	if err != nil {
		return nil, err
	}
	return &GzipFilter{
		pool: &sync.Pool{
			New: func() interface{} {
				w, _ := gzip.NewWriterLevel(ioutil.Discard, level)
				return w
			},
		},
	}, nil
}

func (f *GzipFilter) Invoke(ctx Context, chain FilterChain) {

	if !f.shouldCompress(ctx.Request()) {
		chain.Next(ctx)
		return
	}

	w := f.pool.Get().(*gzip.Writer)
	defer f.pool.Put(w)

	defer w.Reset(ioutil.Discard)
	w.Reset(ctx.Response().Get())

	ctx.SetHeader(HeaderContentEncoding, "gzip")
	ctx.SetHeader(HeaderVary, HeaderAcceptEncoding)

	zw := &gzipWriter{ctx.Response().Get(), w, 0}
	ctx.Response().Set(zw)
	defer func() {
		w.Close()
		ctx.SetHeader(HeaderContentLength, strconv.Itoa(zw.size))
	}()

	chain.Next(ctx)
}

func (f *GzipFilter) shouldCompress(req *http.Request) bool {
	if !strings.Contains(req.Header.Get("Accept-Encoding"), "gzip") ||
		strings.Contains(req.Header.Get("Connection"), "Upgrade") ||
		strings.Contains(req.Header.Get("Accept"), "text/event-stream") {
		return false
	}
	ext := filepath.Ext(req.URL.Path)
	for _, s := range f.ExcludedExtensions {
		if s == ext {
			return false
		}
	}
	for _, s := range f.ExcludedPaths {
		if strings.HasPrefix(req.URL.Path, s) {
			return false
		}
	}
	for _, r := range f.ExcludedPathsRegexes {
		if r.MatchString(req.URL.Path) {
			return false
		}
	}
	return true
}

type gzipWriter struct {
	http.ResponseWriter
	writer *gzip.Writer
	size   int
}

func (g *gzipWriter) WriteHeader(code int) {
	g.Header().Del("Content-Length")
	g.ResponseWriter.WriteHeader(code)
}

func (g *gzipWriter) WriteString(s string) (int, error) {
	g.Header().Del("Content-Length")
	n, err := g.writer.Write([]byte(s))
	g.size += n
	return n, err
}

func (g *gzipWriter) Write(data []byte) (int, error) {
	g.Header().Del("Content-Length")
	n, err := g.writer.Write(data)
	g.size += n
	return n, err
}
