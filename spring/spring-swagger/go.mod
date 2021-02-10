module github.com/go-spring/spring-swagger

go 1.12

require (
	github.com/go-openapi/spec v0.20.2
	github.com/go-spring/spring-web v1.0.5
)

replace (
	github.com/go-spring/spring-web  => ../spring-web
)