module github.com/go-spring/starter-rabbitmq

go 1.12

require (
	github.com/go-spring/spring-core v1.0.5
	github.com/go-spring/spring-stl v0.0.0-20210724145437-4e7cb5d3e0ce
	github.com/streadway/amqp v1.0.0
)

replace (
	github.com/go-spring/spring-core => ../../spring/spring-core
	github.com/go-spring/spring-stl => ../../spring/spring-stl
)
