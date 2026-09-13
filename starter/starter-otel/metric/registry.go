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
	"go-spring.org/starter-otel/internal/registry"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
)

// MeterExporterFactory builds a metric reader (and, for pull-based exporters,
// the scrape artifacts) for one backend from the metrics config. A push-based
// exporter (otlp, stdout) returns only reader and a nil *PromServe; a pull-based
// exporter (prometheus) returns the scrape handler (and optional dedicated
// server) in pull. Register one under a name to add a metrics backend beyond
// the built-ins; NewMeterProvider looks it up by cfg.Exporter.
type MeterExporterFactory func(cfg MetricsConfig) (reader sdkmetric.Reader, pull *PromServe, err error)

// exporters is the shared generic registry (see internal/registry). It holds
// the bookkeeping that used to be duplicated byte-for-byte with the trace
// registry; only the factory type and category string differ.
var exporters = registry.New[MeterExporterFactory]("metric")

// RegisterMeterExporter makes a metric exporter factory available under name.
// It panics on empty name, nil factory, or a duplicate - mirroring the
// driver-registry idiom used elsewhere (starter-otel's own registries, cloud/cache
// RegisterDriver) so a mis-wired or duplicate
// registration fails loudly at init.
func RegisterMeterExporter(name string, f MeterExporterFactory) {
	if f == nil {
		panic("metric: register nil meter exporter factory for " + name)
	}
	exporters.Register(name, f)
}

func lookupMeterExporter(name string) (MeterExporterFactory, bool) {
	return exporters.Lookup(name)
}

// unknownExporterErr builds the error returned when cfg.Exporter names no
// registered metric exporter, listing the available ones so the misconfig is
// self-diagnosing.
func unknownExporterErr(name string) error { return exporters.UnknownErr(name) }
