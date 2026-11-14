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

// observer.go is hertz's observability seam: the server middlewares that emit
// this starter's request-plane signals, and the default OTel-backed adapter.

package StarterHertz

import (
	"github.com/cloudwego/hertz/pkg/app"
	"go-spring.org/spring/gs"
)

// Observer supplies hertz's request-plane middlewares. It is the seam: the
// default is OTel, and an application may contribute its own adapter.
type Observer interface {
	// Metrics returns the middleware that records count, duration and the
	// in-flight gauge.
	Metrics() (app.HandlerFunc, error)
}

// NewOTelObserver returns the default OTel-backed [Observer].
func NewOTelObserver() (Observer, error) { return otelObserver{}, nil }

// otelObserver is the default [Observer]; its middleware bodies live in
// metrics.go and tracing.go.
type otelObserver struct{}

// tracingMiddleware is the standard OTel tracing middleware. Tracing is not
// part of the metric seam.
func tracingMiddleware() app.HandlerFunc { return tracing() }

// The package's default [Observer] bean: an application that contributes its
// own observer takes precedence.
func init() {
	gs.Provide(NewOTelObserver).Condition(gs.OnMissingBean[Observer]()).Caller(1)
}
