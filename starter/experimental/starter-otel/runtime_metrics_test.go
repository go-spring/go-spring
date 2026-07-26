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

package StarterOTel

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"go-spring.org/stdlib/testing/assert"
	runtimemetrics "go.opentelemetry.io/contrib/instrumentation/runtime"
)

// TestRuntimeMetricsExposed is a self-contained smoke test (no docker): it
// builds the pull-based prometheus MeterProvider the starter uses, starts Go
// runtime instrumentation against it, then scrapes the endpoint and asserts the
// runtime metrics actually render. This is the behaviour setup() wires up when
// ${spring.observability.metrics.runtime.enable} is true — proving runtime.*
// metrics reach the exporter with zero per-project code.
func TestRuntimeMetricsExposed(t *testing.T) {
	cfg := MetricsConfig{
		Enable:   true,
		Exporter: "prometheus",
		Port:     19099,
		Path:     "/metrics",
	}

	res, err := newResource("runtime-smoke")
	assert.Error(t, err).Nil()

	mp, ps, err := newMeterProvider(cfg, res)
	assert.Error(t, err).Nil()
	assert.That(t, ps).NotNil()
	assert.That(t, ps.server).NotNil()
	defer func() {
		_ = mp.Shutdown(context.Background())
		_ = ps.server.Shutdown(context.Background())
	}()

	err = runtimemetrics.Start(runtimemetrics.WithMeterProvider(mp))
	assert.Error(t, err).Nil()

	// The scrape server binds asynchronously and the async runtime callbacks
	// only fire on collection, so poll the endpoint until goroutine metrics
	// appear rather than assuming the first request is ready.
	url := fmt.Sprintf("http://127.0.0.1:%d%s", cfg.Port, cfg.Path)
	var body string
	for range 30 {
		resp, err := http.Get(url)
		if err == nil {
			b, _ := io.ReadAll(resp.Body)
			_ = resp.Body.Close()
			body = string(b)
			if strings.Contains(body, "go_goroutine_count") {
				break
			}
		}
		time.Sleep(100 * time.Millisecond)
	}

	// go_goroutine_count is always > 0 and go_processor_limit mirrors GOMAXPROCS
	// — both are stable proof that the Go runtime feed is live on the endpoint.
	assert.String(t, body).Contains("go_goroutine_count")
	assert.String(t, body).Contains("go_processor_limit")
}
