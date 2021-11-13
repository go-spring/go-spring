module github.com/go-spring/starter-go-redis

go 1.14

require (
	github.com/go-redis/redis/v8 v8.11.4
	github.com/go-spring/spring-base v1.1.0-rc1.0.20211113013245-ace7b4d73418
	github.com/go-spring/spring-core v1.1.0-rc1.0.20211113013626-adb083a077e7
	github.com/go-spring/spring-go-redis v1.1.0-rc1.0.20211113014651-a9d009f833d6
	github.com/go-spring/starter-core v1.1.0-rc1
)

//replace (
//	github.com/go-spring/spring-core => ../../spring/spring-core
//	github.com/go-spring/spring-base => ../../spring/spring-base
//	github.com/go-spring/starter-core => ../starter-core
//)
