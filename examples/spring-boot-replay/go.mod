module github.com/go-spring/examples/spring-boot-replay

go 1.14

require (
	github.com/go-spring/spring-base v1.1.0-rc1
	github.com/go-spring/spring-core v1.1.0-rc1
	github.com/go-spring/starter-echo v1.1.0-rc1
	github.com/go-spring/starter-go-redis v1.1.0-rc1
)

replace (
	github.com/go-spring/spring-base => ../../spring/spring-base
	github.com/go-spring/spring-core => ../../spring/spring-core
	github.com/go-spring/starter-web => ../../starter/starter-web
	github.com/go-spring/starter-go-redis => ../../starter/starter-go-redis
)
