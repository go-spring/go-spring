module github.com/go-spring/starter-go-redis

go 1.12

require (
	github.com/elliotchance/redismock v1.5.3
	github.com/go-redis/redis v6.15.9+incompatible
	github.com/go-spring/spring-core v1.0.5
	github.com/go-spring/starter-core v0.0.0-20210215012223-32c9b94871eb
	github.com/stretchr/objx v0.3.0 // indirect
)

replace (
	github.com/go-spring/starter-core => ../starter-core
	github.com/go-spring/spring-core => ../../spring/spring-core
)