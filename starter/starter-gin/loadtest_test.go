/*
 * Copyright 2025 The Go-Spring Authors.
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

package StarterGin

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"go-spring.org/cloud/traffic"
	"go-spring.org/stdlib/testing/assert"
)

func TestLoadTestMiddleware_TagsContextFromHeader(t *testing.T) {
	var saw bool
	prop, err := traffic.NewDefaultPropagator(traffic.DefaultBinding())
	assert.Error(t, err).Nil()
	e := gin.New()
	e.Use(LoadTest(prop))
	e.GET("/x", func(c *gin.Context) {
		saw = prop.IsLoadTest(c.Request.Context())
		c.Status(http.StatusOK)
	})

	// With the marker header: handler sees a load-test context.
	req := httptest.NewRequest(http.MethodGet, "/x", nil)
	req.Header.Set("X-LoadTest", "1")
	e.ServeHTTP(httptest.NewRecorder(), req)
	assert.That(t, saw).True()

	// Without it: plain context.
	saw = false
	req2 := httptest.NewRequest(http.MethodGet, "/x", nil)
	e.ServeHTTP(httptest.NewRecorder(), req2)
	assert.That(t, saw).False()
}

func TestLoadTestMiddleware_RebasedHeaderAndTruthyValues(t *testing.T) {
	var saw bool
	// The header a hop reads belongs to the propagator, so a company re-bases it
	// there (not in the middleware config).
	b := traffic.DefaultBinding()
	b.Key = "X-Stress"
	prop, err := traffic.NewDefaultPropagator(b)
	assert.Error(t, err).Nil()
	e := gin.New()
	e.Use(LoadTest(prop))
	e.GET("/x", func(c *gin.Context) {
		saw = prop.IsLoadTest(c.Request.Context())
	})

	// Custom header, the wire value matches.
	for _, v := range []string{"1"} {
		saw = false
		req := httptest.NewRequest(http.MethodGet, "/x", nil)
		req.Header.Set("X-Stress", v)
		e.ServeHTTP(httptest.NewRecorder(), req)
		assert.That(t, saw).True()
	}
	// Any other value is real traffic.
	for _, v := range []string{"true", "ON", "0"} {
		saw = false
		req := httptest.NewRequest(http.MethodGet, "/x", nil)
		req.Header.Set("X-Stress", v)
		e.ServeHTTP(httptest.NewRecorder(), req)
		assert.That(t, saw).False()
	}
	// Non-truthy value does not tag.
	saw = false
	req := httptest.NewRequest(http.MethodGet, "/x", nil)
	req.Header.Set("X-Stress", "0")
	e.ServeHTTP(httptest.NewRecorder(), req)
	assert.That(t, saw).False()

	// The default header does NOT match when a custom one is configured.
	saw = false
	req2 := httptest.NewRequest(http.MethodGet, "/x", nil)
	req2.Header.Set("X-LoadTest", "1")
	e.ServeHTTP(httptest.NewRecorder(), req2)
	assert.That(t, saw).False()
}
