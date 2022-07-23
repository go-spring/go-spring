module github.com/go-spring/examples/spring-boot-demo

go 1.14

require (
	github.com/go-spring/spring-base v1.1.0-rc4.0.20220723014310-0954983fa7c4
	github.com/go-spring/spring-core v1.1.0-rc4.0.20220723014617-b854e072484a
	github.com/go-spring/starter-echo v1.1.0-rc4.0.20220723015757-2eeca61cad48
	github.com/go-spring/starter-go-redis v1.1.0-rc4.0.20220723015846-2cdc6959f4b8
	github.com/go-spring/starter-gorm v1.1.0-rc4.0.20220723015857-3edc6a9569d2
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
