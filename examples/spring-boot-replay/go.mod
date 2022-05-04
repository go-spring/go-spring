module github.com/go-spring/examples/spring-boot-replay

go 1.14

require (
	github.com/go-spring/spring-base v1.1.0-rc3.0.20220504021136-8ace13a580ab
	github.com/go-spring/spring-core v1.1.0-rc3.0.20220504021535-bb85dfe5bd69
	github.com/go-spring/starter-echo v1.1.0-rc3.0.20220504023121-9ef6e4c95ce6 // indirect
	github.com/go-spring/starter-gin v1.1.0-rc3.0.20220504023133-83d88ae5fb7d
	github.com/go-spring/starter-go-redis v1.1.0-rc3.0.20220504023155-c3b3d5065ad9
)

//replace (
//	github.com/go-spring/spring-base => ../../spring/spring-base
//	github.com/go-spring/spring-core => ../../spring/spring-core
//	github.com/go-spring/spring-echo => ../../spring/spring-echo
//	github.com/go-spring/spring-gin => ../../spring/spring-gin
//	github.com/go-spring/spring-go-redis => ../../spring/spring-go-redis
//	github.com/go-spring/starter-go-redis => ../../starter/starter-go-redis
//)
