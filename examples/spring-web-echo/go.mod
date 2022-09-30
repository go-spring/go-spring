module github.com/go-spring/examples/spring-web-echo

go 1.16

require (
	github.com/go-spring/spring-base v1.1.2
	github.com/go-spring/spring-core v1.1.2
	github.com/go-spring/spring-echo v1.1.2
)

replace (
	github.com/go-spring/spring-base => ../../spring/spring-base
	github.com/go-spring/spring-core => ../../spring/spring-core
	github.com/go-spring/spring-echo => ../../spring/spring-echo
)
