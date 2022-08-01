module github.com/go-spring/starter-gin

go 1.14

require (
	github.com/go-spring/spring-core v1.1.0
	github.com/go-spring/spring-gin v1.1.0
)

replace (
	github.com/go-spring/spring-base => ../../spring/spring-base
	github.com/go-spring/spring-core => ../../spring/spring-core
	github.com/go-spring/spring-gin => ../../spring/spring-gin
)
