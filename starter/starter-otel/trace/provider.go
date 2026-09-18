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

package trace

import (
	"fmt"
	"strings"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
)

// NewResource builds the process-level resource every span and metric carries:
// the OTel default resource -- the telemetry.sdk.* attributes and whatever
// OTEL_RESOURCE_ATTRIBUTES declares -- overlaid with service.name. This is how
// a process attaches its own dimensions (deployment environment, cluster,
// tenant, version) without go-spring defining an API for it: the mechanism is
// OTel's, so no per-component adaptation is needed.
//
// OTEL_SERVICE_NAME, when set, wins over serviceName: the environment variable
// is the operator-level override, which is also OTel's own precedence (its
// fromEnv detector runs after the fallback name). Without it the configured
// name is used.
//
// The default resource is merged FIRST so go-spring's own attribute wins: the
// default always carries a service.name of its own -- OTel's
// "unknown_service:<binary>" fallback -- so merging it last would clobber the
// configured name on every process that does not set OTEL_SERVICE_NAME. The
// explicit environment read above is the other half of the rule: it is what
// lets OTEL_SERVICE_NAME win over the configured name rather than the reverse.
func NewResource(serviceName string) (*resource.Resource, error) {
	env := resource.Environment().Set()
	if v, ok := env.Value(attribute.Key("service.name")); ok && v.AsString() != "" {
		serviceName = v.AsString()
	}
	return resource.Merge(resource.Default(), resource.NewSchemaless(
		attribute.String("service.name", serviceName),
	))
}

// NewTracerProvider builds a batching TracerProvider for the configured
// exporter. The exporter is looked up by name in the span-exporter registry;
// the built-in names (otlp-grpc, otlp-http, stdout) are registered by subpackages
// the starter blank-imports, and applications may add their own via
// RegisterSpanExporter. Endpoint is required for the otlp exporters; an empty
// endpoint falls back to the exporter's own default (localhost:4317 / :4318).
func NewTracerProvider(cfg TraceConfig, res *resource.Resource) (*sdktrace.TracerProvider, error) {
	if cfg.SamplerRatio <= 0 {
		return nil, fmt.Errorf(
			"observability: trace.sampler-ratio=%v is invalid: a non-positive ratio drops every trace (config default is 1.0); to disable tracing set spring.observability.trace.exporter=none or spring.observability.trace.enable=false", cfg.SamplerRatio)
	}
	f, ok := lookupSpanExporter(cfg.Exporter)
	if !ok {
		return nil, unknownExporterErr(cfg.Exporter)
	}
	exp, err := f(cfg)
	if err != nil {
		return nil, err
	}

	return sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(exp),
		// Applies the attributes a context carries (see
		// observability.WithContextAttributes) to every span in the process,
		// including the ones instrumentation starts inside the framework.
		// Registered alongside the batcher: the SDK keeps a list of processors,
		// and this one holds no resources.
		sdktrace.WithSpanProcessor(ContextAttributesProcessor()),
		// Tags every span of a load-test request, so synthetic traffic is
		// separable from production traffic in traces and metrics. Reads the
		// traffic contract; the marker package itself stays passive.
		sdktrace.WithSpanProcessor(LoadTestProcessor()),
		sdktrace.WithResource(res),
		sdktrace.WithSampler(NewSampler(cfg.SamplerRatio)),
	), nil
}

// NewSampler maps a ratio to a ParentBased sampler: >=1 always sample, a
// value in (0,1) a TraceID ratio sampler. ParentBased keeps a trace's
// sampling decision consistent once an upstream service has decided.
// Non-positive ratios are rejected by NewTracerProvider instead of being
// silently turned into "never sample" (which would drop everything).
func NewSampler(ratio float64) sdktrace.Sampler {
	switch {
	case ratio >= 1:
		return sdktrace.ParentBased(sdktrace.AlwaysSample())
	default:
		return sdktrace.ParentBased(sdktrace.TraceIDRatioBased(ratio))
	}
}

// NewPropagator returns the text-map propagator for cross-service context
// propagation named by spec, which is a comma-separated list of registered
// propagator names composed in order. The built-in names are "tracecontext"
// and "baggage" (pre-registered at init); "w3c" is an alias for that pair.
// "none" returns nil, leaving the process global untouched. An empty spec
// defaults to "w3c". A company composes its own propagators into the fleet by
// RegisterPropagator(name, p) and naming them here, e.g.
// "w3c,luohua" — so its named business headers ride every transport.
func NewPropagator(spec string) (propagation.TextMapPropagator, error) {
	if strings.TrimSpace(spec) == "" {
		spec = "w3c"
	}
	if strings.TrimSpace(spec) == "none" {
		return nil, nil
	}

	// Expand the spec into concrete registered names ("w3c" widens to the pair).
	var names []string
	for _, tok := range strings.Split(spec, ",") {
		tok = strings.TrimSpace(tok)
		if tok == "" {
			continue
		}
		if tok == "w3c" {
			names = append(names, "tracecontext", "baggage")
			continue
		}
		names = append(names, tok)
	}

	var parts []propagation.TextMapPropagator
	for _, n := range names {
		p, ok := lookupPropagator(n)
		if !ok {
			return nil, fmt.Errorf("observability: unknown propagator %q (registered: %s)",
				n, strings.Join(propagatorNames(), ", "))
		}
		parts = append(parts, p)
	}
	if len(parts) == 0 {
		return nil, fmt.Errorf("observability: empty propagator spec %q", spec)
	}
	if len(parts) == 1 {
		return parts[0], nil
	}
	return propagation.NewCompositeTextMapPropagator(parts...), nil
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
	Propagator   string  `value:"${propagator:=w3c}"` // w3c[,<extra>...] | <name>[,<name>...] | none; see NewPropagator
}
