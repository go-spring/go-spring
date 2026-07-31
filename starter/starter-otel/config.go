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
	"go-spring.org/starter-otel/metric"
	"go-spring.org/starter-otel/trace"
)

// Config is the single, framework-level observability configuration bound to
// ${spring.observability}. It is the one place trace/metrics providers are
// defined; every instrumented component (gorm, redis, http, ...) consumes the
// providers through the OTel process globals set by this starter, so a project
// configures observability once here instead of adapting each component.
type Config struct {
	Enable      bool                `value:"${enable:=true}"`
	ServiceName string              `value:"${service-name:=${spring.application.name:=go-spring-app}}"`
	Trace       trace.TraceConfig   `value:"${trace}"`
	Metrics     metric.MetricsConfig `value:"${metrics}"`
}