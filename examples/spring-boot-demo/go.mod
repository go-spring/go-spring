module github.com/go-spring/examples/spring-boot-demo

go 1.16

require (
	github.com/go-spring/spring-base v1.1.2-0.20220912232223-ad27a0e73218
	github.com/go-spring/spring-core v1.1.2-0.20220912232602-72c7fd83f924
	github.com/go-spring/starter-echo v1.1.2-0.20220912233959-d3e29471eb9a
	github.com/go-spring/starter-go-redis v1.1.2-0.20220912234038-802230e38aee
	github.com/go-spring/starter-gorm v1.1.2-0.20220912234049-e2fcf01545f6
	github.com/labstack/echo/v4 v4.6.1
	github.com/spf13/viper v1.3.1
	go.mongodb.org/mongo-driver v1.7.3
	gorm.io/gorm v1.22.4
)

//replace (
//	github.com/go-spring/spring-base => ../../spring/spring-base
//	github.com/go-spring/spring-core => ../../spring/spring-core
//	github.com/go-spring/spring-echo => ../../spring/spring-echo
//	github.com/go-spring/starter-echo => ../../starter/starter-echo
//	github.com/go-spring/starter-gorm => ../../starter/starter-gorm
//	github.com/go-spring/spring-go-redis => ../../spring/spring-go-redis
//	github.com/go-spring/starter-go-redis => ../../starter/starter-go-redis
//)
