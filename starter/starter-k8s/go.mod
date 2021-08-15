module github.com/go-spring/starter-grpc

go 1.14

require (
	github.com/go-spring/spring-boost v1.1.0-alpha // indirect
	github.com/go-spring/spring-core v1.1.0-alpha
	github.com/spf13/viper v1.6.1
)

replace (
	github.com/go-spring/spring-boost => ../../spring/spring-boost
	github.com/go-spring/spring-core => ../../spring/spring-core
)
