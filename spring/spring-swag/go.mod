module github.com/go-spring/spring-swag

go 1.14

require (
	github.com/go-openapi/spec v0.20.4
	github.com/go-spring/spring-base v1.1.0-rc4
	github.com/go-spring/spring-core v1.1.0-rc4
	github.com/swaggo/http-swagger v1.1.2
)

replace (
	github.com/go-spring/spring-core => ../spring-core
	github.com/go-spring/spring-base => ../spring-base
)
