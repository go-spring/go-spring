module github.com/go-spring/examples/spring-boot-hello

go 1.14

require (
	github.com/go-spring/spring-core v1.1.0-rc2
	github.com/go-spring/starter-echo v1.1.0-rc2
)

replace (
	github.com/go-spring/spring-core => ../../spring/spring-core
	github.com/go-spring/spring-echo => ../../spring/spring-echo
	github.com/go-spring/starter-echo => ../../starter/starter-echo
)
