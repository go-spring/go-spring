module github.com/go-spring/examples/spring-web-testcases

go 1.14

require (
	github.com/go-openapi/spec v0.20.4
	github.com/go-spring/spring-base v1.1.0-rc2
	github.com/go-spring/spring-core v1.1.0-rc2
	github.com/go-spring/spring-echo v1.1.0-rc2
	github.com/go-spring/spring-swag v1.1.0-rc2
)

replace (
	github.com/go-spring/spring-base => ../../spring/spring-base
	github.com/go-spring/spring-core => ../../spring/spring-core
	github.com/go-spring/spring-echo => ../../spring/spring-echo
	github.com/go-spring/spring-swag => ../../spring/spring-swag
)
