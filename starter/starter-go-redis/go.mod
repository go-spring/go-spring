module github.com/go-spring/starter-go-redis

go 1.14

require (
	github.com/go-redis/redis/v8 v8.11.4
	github.com/go-spring/spring-base v1.1.0-rc1.0.20211030141542-4d3ac8ed9e93
	github.com/go-spring/spring-core v1.1.0-rc1.0.20211031004208-3997c4df4f37
	github.com/go-spring/spring-go-redis v1.1.0-rc1.0.20211031023534-7193bdd294e2
	github.com/go-spring/starter-core v1.1.0-rc1
)

//replace (
//	github.com/go-spring/spring-core => ../../spring/spring-core
//	github.com/go-spring/spring-base => ../../spring/spring-base
//	github.com/go-spring/starter-core => ../starter-core
//)
