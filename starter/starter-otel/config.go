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

import "time"

// Config is the single, framework-level observability configuration bound to
// ${spring.observability}. It is the one place trace/metrics providers are
// defined; every instrumented component (gorm, redis, http, ...) consumes the
// providers through the OTel process globals set by this starter, so a project
// configures observability once here instead of adapting each component.
type Config struct {
	Enable      bool          `value:"${enable:=true}"`
	ServiceName string        `value:"${service-name:=${spring.application.name:=go-spring-app}}"`
	Trace       TraceConfig   `value:"${trace}"`
	Metrics     MetricsConfig `value:"${metrics}"`
}

// TraceConfig configures the shared TracerProvider under
// ${spring.observability.trace}. Exporter selects the span backend; Endpoint is
// required for the otlp exporters. SamplerRatio drives a ParentBased ratio
// sampler (>=1 always, <=0 never). Empty/zero values keep OTel SDK defaults.
type TraceConfig struct {
	Enable       bool    `value:"${enable:=true}"`
	Exporter     string  `value:"${exporter:=otlp-grpc}"` // otlp-grpc|otlp-http|stdout|none
	Endpoint     string  `value:"${endpoint:=}"`
	Insecure     bool    `value:"${insecure:=true}"`
	SamplerRatio float64 `value:"${sampler-ratio:=1.0}"`
	Propagator   string  `value:"${propagator:=w3c}"` // w3c|none
}

// MetricsConfig configures the shared MeterProvider under
// ${spring.observability.metrics}. The prometheus exporter is pull-based and
// serves Path on Port; the otlp/stdout exporters are push-based on Interval.
// Empty/zero values keep OTel SDK defaults.
type MetricsConfig struct {
	Enable   bool          `value:"${enable:=true}"`
	Exporter string        `value:"${exporter:=otlp-grpc}"` // otlp-grpc|otlp-http|prometheus|stdout|none
	Endpoint string        `value:"${endpoint:=}"`
	Insecure bool          `value:"${insecure:=true}"`
	Port     int           `value:"${port:=9090}"`
	Path     string        `value:"${path:=/metrics}"`
	Interval time.Duration `value:"${interval:=10s}"` // push interval for otlp/stdout readers
}
