module github.com/go-spring/examples/spring-boot-replay

go 1.14

require (
	github.com/go-spring/spring-base v1.1.0-rc3
	github.com/go-spring/spring-core v1.1.0-rc3
	github.com/go-spring/starter-echo v1.1.0-rc3 // indirect
	github.com/go-spring/starter-gin v1.1.0-rc3
	github.com/go-spring/starter-go-redis v1.1.0-rc3
)

replace (
	github.com/go-spring/spring-base => ../../spring/spring-base
	github.com/go-spring/spring-core => ../../spring/spring-core
	github.com/go-spring/spring-echo => ../../spring/spring-echo
	github.com/go-spring/spring-gin => ../../spring/spring-gin
	github.com/go-spring/spring-go-redis => ../../spring/spring-go-redis
	github.com/go-spring/starter-go-redis => ../../starter/starter-go-redis
)
