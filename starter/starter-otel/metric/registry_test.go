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

package metric

import (
	"testing"

	"go.opentelemetry.io/otel/sdk/resource"
	"go-spring.org/stdlib/testing/assert"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
)

// TestRegisterMeterExporter proves a custom metric exporter registered under a
// new name is resolved by NewMeterProvider (and its *PromServe forwarded), and
// that an unknown name yields an error listing the registered exporters.
func TestRegisterMeterExporter(t *testing.T) {
	const name = "test-noop"
	wantPull := &PromServe{}
	RegisterMeterExporter(name, func(_ MetricsConfig) (sdkmetric.Reader, *PromServe, error) {
		// A manual reader is the simplest standalone Reader for a test.
		return sdkmetric.NewManualReader(), wantPull, nil
	})
	t.Cleanup(func() {
		exporterMu.Lock()
		delete(exporterReg, name)
		exporterMu.Unlock()
	})

	res := resource.NewSchemaless()

	mp, ps, err := NewMeterProvider(MetricsConfig{Exporter: name}, res)
	assert.Error(t, err).Nil()
	assert.That(t, mp).NotNil()
	assert.That(t, ps).Same(wantPull)
	_ = mp.Shutdown(nil)

	// Unknown exporter yields an error naming the registered set.
	_, _, err = NewMeterProvider(MetricsConfig{Exporter: "does-not-exist"}, res)
	assert.Error(t, err).NotNil()
	assert.String(t, err.Error()).Contains(name)
}
