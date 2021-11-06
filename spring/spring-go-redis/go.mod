module github.com/go-spring/spring-go-redis

go 1.14

require (
	github.com/go-redis/redis/v8 v8.11.4
	github.com/go-spring/spring-base v1.1.0-rc1.0.20211030141542-4d3ac8ed9e93 // indirect
	github.com/go-spring/spring-core v1.1.0-rc1.0.20211031004208-3997c4df4f37
)

replace (
	github.com/go-spring/spring-base => ../spring-base
	github.com/go-spring/spring-core => ../spring-core
)
