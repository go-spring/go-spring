module go-spring.org/starter-config-bus/example

go 1.26.1

require (
	github.com/nats-io/nats.go v1.38.0
	go-spring.org/log v0.1.4
	go-spring.org/spring v1.3.4
	go-spring.org/starter-config-bus v0.0.0
	go-spring.org/starter-nats v0.0.0
)

replace go-spring.org/starter-config-bus => ../

replace go-spring.org/starter-nats => ../../starter-nats
