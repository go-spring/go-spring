module github.com/go-spring/starter-go-redis

go 1.14

require (
	github.com/go-spring/spring-base v1.1.0
	github.com/go-spring/spring-core v1.1.0
	github.com/go-spring/spring-go-redis v1.1.0
)

replace (
	github.com/go-spring/spring-base => ../../spring/spring-base
	github.com/go-spring/spring-core => ../../spring/spring-core
	github.com/go-spring/spring-go-redis => ../../spring/spring-go-redis
)
