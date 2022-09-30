module github.com/go-spring/examples/spring-iris-server

go 1.14

require (
	github.com/go-spring/spring-base v1.1.2
	github.com/go-spring/spring-core v1.1.2
	github.com/kataras/iris/v12 v12.2.0-alpha8
)

replace (
	github.com/go-spring/spring-base => ../../spring/spring-base
	github.com/go-spring/spring-core => ../../spring/spring-core
)
