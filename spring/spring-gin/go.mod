module github.com/go-spring/spring-gin

go 1.14

require (
	github.com/gin-gonic/gin v1.7.4
	github.com/go-spring/spring-base v1.1.2
	github.com/go-spring/spring-core v1.1.2
)

replace (
	github.com/go-spring/spring-base => ../spring-base
	github.com/go-spring/spring-core => ../spring-core
)
