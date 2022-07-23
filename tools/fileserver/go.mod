module github.com/go-spring/go-spring/tools/fileserver

go 1.14

require (
	github.com/go-spring/spring-core v1.1.0-rc4
	github.com/go-spring/spring-echo v1.1.0-rc4
)

replace (
	github.com/go-spring/spring-base => ../../spring/spring-base
	github.com/go-spring/spring-core => ../../spring/spring-core
	github.com/go-spring/spring-echo => ../../spring/spring-echo
)
