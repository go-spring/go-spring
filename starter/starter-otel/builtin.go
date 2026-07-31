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

// The blank imports below register the built-in trace and metrics exporters
// with their respective driver registries (trace.RegisterSpanExporter,
// metric.RegisterMeterExporter) at init. Importing starter-otel alone is enough
// to make otlp-grpc (the default), otlp-http, prometheus, and stdout available.
//
// An application that wants to slim its dependency footprint can stop importing
// the starters it does not need (e.g. drop metric/prometheus to avoid pulling
// the Prometheus client) and/or register its own exporter via the registry.
import (
	_ "go-spring.org/starter-otel/metric/otlp"
	_ "go-spring.org/starter-otel/metric/prometheus"
	_ "go-spring.org/starter-otel/metric/stdout"
	_ "go-spring.org/starter-otel/trace/otlp"
	_ "go-spring.org/starter-otel/trace/stdout"
)
