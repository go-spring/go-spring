module github.com/go-spring/starter-rabbit

go 1.14

require (
	github.com/go-spring/spring-base v1.1.0
	github.com/go-spring/spring-core v1.1.0
	github.com/streadway/amqp v1.0.0
)

replace (
	github.com/go-spring/spring-base => ../../spring/spring-base
	github.com/go-spring/spring-core => ../../spring/spring-core
)
