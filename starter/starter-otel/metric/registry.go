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
	"fmt"
	"sort"
	"strings"
	"sync"

	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
)

// MeterExporterFactory builds a metric reader (and, for pull-based exporters,
// the scrape artifacts) for one backend from the metrics config. A push-based
// exporter (otlp, stdout) returns only reader and a nil *PromServe; a pull-based
// exporter (prometheus) returns the scrape handler (and optional dedicated
// server) in pull. Register one under a name to add a metrics backend beyond
// the built-ins; NewMeterProvider looks it up by cfg.Exporter.
type MeterExporterFactory func(cfg MetricsConfig) (reader sdkmetric.Reader, pull *PromServe, err error)

var (
	exporterMu  sync.RWMutex
	exporterReg = map[string]MeterExporterFactory{}
)

// RegisterMeterExporter makes a metric exporter factory available under name.
// It panics on empty name, nil factory, or a duplicate - mirroring the
// driver-registry idiom used elsewhere (discovery.Register, starter-go-redis
// RegisterDriver) so a mis-wired or duplicate registration fails loudly at init.
func RegisterMeterExporter(name string, f MeterExporterFactory) {
	if name == "" {
		panic("metric: register meter exporter with empty name")
	}
	if f == nil {
		panic("metric: register nil meter exporter factory for " + name)
	}
	exporterMu.Lock()
	defer exporterMu.Unlock()
	if _, ok := exporterReg[name]; ok {
		panic("metric: meter exporter already registered: " + name)
	}
	exporterReg[name] = f
}

func lookupMeterExporter(name string) (MeterExporterFactory, bool) {
	exporterMu.RLock()
	defer exporterMu.RUnlock()
	f, ok := exporterReg[name]
	return f, ok
}

// registeredMeterExporters returns the sorted names of all registered metric
// exporters, for inclusion in "unknown exporter" error messages.
func registeredMeterExporters() []string {
	exporterMu.RLock()
	defer exporterMu.RUnlock()
	names := make([]string, 0, len(exporterReg))
	for n := range exporterReg {
		names = append(names, n)
	}
	sort.Strings(names)
	return names
}

// unknownExporterErr builds the error returned when cfg.Exporter names no
// registered metric exporter, listing the available ones so the misconfig is
// self-diagnosing.
func unknownExporterErr(name string) error {
	return fmt.Errorf("observability: unknown metrics exporter %q (registered: %s)",
		name, strings.Join(registeredMeterExporters(), ", "))
}
