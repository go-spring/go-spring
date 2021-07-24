module github.com/go-spring/spring-gin

go 1.12

require (
	github.com/gin-gonic/gin v1.6.3
	github.com/go-spring/spring-core v1.0.5
	github.com/go-spring/spring-stl v0.0.0-20210724145437-4e7cb5d3e0ce
)

replace (
	github.com/go-spring/spring-core => ../spring-core
	github.com/go-spring/spring-stl => ../spring-stl
)