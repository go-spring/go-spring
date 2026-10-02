module go-spring.org/cloud/observability/example-otel

go 1.26.1

require (
	go-spring.org/cloud v0.0.0
	go-spring.org/stdlib v0.1.7
	go.opentelemetry.io/otel v1.45.0
	go.opentelemetry.io/otel/sdk v1.45.0
	go.opentelemetry.io/otel/trace v1.45.0
)

replace go-spring.org/cloud => ../..
