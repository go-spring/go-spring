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
	"go-spring.org/cloud/governance/traffic"
	"go-spring.org/cloud/governance/traffic/canonical"
	"go-spring.org/stdlib/testing/assert"
)

func TestLoadTestMiddleware_TagsContextFromHeader(t *testing.T) {
	var saw bool
	e := gin.New()
	e.Use(LoadTest(""))
	e.GET("/x", func(c *gin.Context) {
		saw = traffic.IsLoadTest(c.Request.Context())
		c.Status(http.StatusOK)
	})

	// With the marker header: handler sees a load-test context.
	req := httptest.NewRequest(http.MethodGet, "/x", nil)
	req.Header.Set(canonical.HeaderLoadTest, "1")
	e.ServeHTTP(httptest.NewRecorder(), req)
	assert.That(t, saw).True()

	// Without it: plain context.
	saw = false
	req2 := httptest.NewRequest(http.MethodGet, "/x", nil)
	e.ServeHTTP(httptest.NewRecorder(), req2)
	assert.That(t, saw).False()
}

func TestLoadTestMiddleware_CustomHeaderAndTruthyValues(t *testing.T) {
	var saw bool
	e := gin.New()
	e.Use(LoadTest("X-Stress"))
	e.GET("/x", func(c *gin.Context) {
		saw = traffic.IsLoadTest(c.Request.Context())
	})

	// Custom header, truthy spellings all match.
	for _, v := range []string{"1", "true", "ON", "Yes", "t"} {
		saw = false
		req := httptest.NewRequest(http.MethodGet, "/x", nil)
		req.Header.Set("X-Stress", v)
		e.ServeHTTP(httptest.NewRecorder(), req)
		assert.That(t, saw).True()
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
	req2.Header.Set(canonical.HeaderLoadTest, "1")
	e.ServeHTTP(httptest.NewRecorder(), req2)
	assert.That(t, saw).False()
}
