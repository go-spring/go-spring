module go-spring.org/cloud/lock/example

go 1.26.1

require (
	go-spring.org/cloud v0.0.0
	go-spring.org/spring v1.3.4
	go-spring.org/starter-lock-memory v0.0.0
	go-spring.org/stdlib v0.1.7
	go-spring.org/log v0.1.4
)

require (
	github.com/antlr4-go/antlr/v4 v4.13.1 // indirect
	github.com/cespare/xxhash/v2 v2.3.0 // indirect
	github.com/go-logr/logr v1.4.4 // indirect
	github.com/go-logr/stdr v1.2.2 // indirect
	go.opentelemetry.io/auto/sdk v1.2.1 // indirect
	go.opentelemetry.io/otel v1.45.0 // indirect
	go.opentelemetry.io/otel/metric v1.45.0 // indirect
	go.opentelemetry.io/otel/trace v1.45.0 // indirect
	golang.org/x/exp v0.0.0-20260410095643-746e56fc9e2f // indirect
)

replace go-spring.org/cloud => ../..

replace go-spring.org/starter-lock-memory => ../../../starter/experimental/starter-lock-memory
