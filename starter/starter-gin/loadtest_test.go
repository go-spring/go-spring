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
	"go-spring.org/cloud/governance/capability/traffic"
	"go-spring.org/stdlib/testing/assert"
)

func TestLoadTestMiddleware_TagsContextFromHeader(t *testing.T) {
	var saw bool
	prop := traffic.NewDefaultPropagator(traffic.DefaultBinding())
	e := gin.New()
	e.Use(LoadTest(prop))
	e.GET("/x", func(c *gin.Context) {
		saw = traffic.IsLoadTest(c.Request.Context(), prop)
		c.Status(http.StatusOK)
	})

	// With the marker header: handler sees a load-test context.
	req := httptest.NewRequest(http.MethodGet, "/x", nil)
	req.Header.Set("X-Traffic", "loadtest")
	e.ServeHTTP(httptest.NewRecorder(), req)
	assert.That(t, saw).True()

	// Without it: plain context.
	saw = false
	req2 := httptest.NewRequest(http.MethodGet, "/x", nil)
	e.ServeHTTP(httptest.NewRecorder(), req2)
	assert.That(t, saw).False()
}

func TestLoadTestMiddleware_RebasedHeaderAndOtherKinds(t *testing.T) {
	var saw bool
	// The header a hop reads belongs to the propagator, so a company re-bases it
	// there (not in the middleware config).
	b := traffic.DefaultBinding()
	b.Key = "X-Stress"
	prop := traffic.NewDefaultPropagator(b)
	e := gin.New()
	e.Use(LoadTest(prop))
	e.GET("/x", func(c *gin.Context) {
		saw = traffic.IsLoadTest(c.Request.Context(), prop)
	})

	// Custom header, the load-test label under it.
	for _, v := range []string{"loadtest"} {
		saw = false
		req := httptest.NewRequest(http.MethodGet, "/x", nil)
		req.Header.Set("X-Stress", v)
		e.ServeHTTP(httptest.NewRecorder(), req)
		assert.That(t, saw).True()
	}
	// Any other label is a different kind, not load-test traffic.
	for _, v := range []string{"true", "ON", "0"} {
		saw = false
		req := httptest.NewRequest(http.MethodGet, "/x", nil)
		req.Header.Set("X-Stress", v)
		e.ServeHTTP(httptest.NewRecorder(), req)
		assert.That(t, saw).False()
	}
	// Another label does not classify as load-test traffic.
	saw = false
	req := httptest.NewRequest(http.MethodGet, "/x", nil)
	req.Header.Set("X-Stress", "0")
	e.ServeHTTP(httptest.NewRecorder(), req)
	assert.That(t, saw).False()

	// The default header does NOT match when a custom one is configured.
	saw = false
	req2 := httptest.NewRequest(http.MethodGet, "/x", nil)
	req2.Header.Set("X-Traffic", "loadtest")
	e.ServeHTTP(httptest.NewRecorder(), req2)
	assert.That(t, saw).False()
}
