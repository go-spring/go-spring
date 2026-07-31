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

// Package otlp registers the OTLP metric exporters (otlp-grpc, otlp-http) with
// the metric meter-exporter registry. It is blank-imported by the top-level
// starter-otel package so the default exporter is available with no per-app
// wiring; an application that wants neither OTLP exporter can drop this import.
package otlp

import (
	"context"

	"go-spring.org/starter-otel/metric"
	"go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetricgrpc"
	"go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetrichttp"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
)

func init() {
	metric.RegisterMeterExporter("otlp-grpc", newGRPC)
	metric.RegisterMeterExporter("otlp-http", newHTTP)
}

// newGRPC builds an OTLP/gRPC metric exporter wrapped in a periodic reader from
// the metrics config. Endpoint is optional - empty falls back to the exporter's
// localhost:4317 default.
func newGRPC(cfg metric.MetricsConfig) (sdkmetric.Reader, *metric.PromServe, error) {
	opts := []otlpmetricgrpc.Option{}
	if cfg.Endpoint != "" {
		opts = append(opts, otlpmetricgrpc.WithEndpoint(cfg.Endpoint))
	}
	if cfg.Insecure {
		opts = append(opts, otlpmetricgrpc.WithInsecure())
	}
	exp, err := otlpmetricgrpc.New(context.Background(), opts...)
	if err != nil {
		return nil, nil, err
	}
	return metric.NewPeriodicReader(exp, cfg.Interval), nil, nil
}

// newHTTP builds an OTLP/HTTP metric exporter wrapped in a periodic reader from
// the metrics config. Endpoint is optional - empty falls back to the exporter's
// localhost:4318 default.
func newHTTP(cfg metric.MetricsConfig) (sdkmetric.Reader, *metric.PromServe, error) {
	opts := []otlpmetrichttp.Option{}
	if cfg.Endpoint != "" {
		opts = append(opts, otlpmetrichttp.WithEndpoint(cfg.Endpoint))
	}
	if cfg.Insecure {
		opts = append(opts, otlpmetrichttp.WithInsecure())
	}
	exp, err := otlpmetrichttp.New(context.Background(), opts...)
	if err != nil {
		return nil, nil, err
	}
	return metric.NewPeriodicReader(exp, cfg.Interval), nil, nil
}
