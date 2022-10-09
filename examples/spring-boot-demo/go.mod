module github.com/go-spring/examples/spring-boot-demo

go 1.16

require (
	github.com/go-spring/spring-base v1.1.3-0.20221009074117-5fc71d4a6063
	github.com/go-spring/spring-core v1.1.3-0.20221009074341-cea99bf58de3
	github.com/go-spring/starter-echo v1.1.3-0.20221009090359-8a65ea13d51f
	github.com/go-spring/starter-go-redis v1.1.3-0.20221009090453-99e7b7960eb4
	github.com/go-spring/starter-gorm v1.1.3-0.20221009091125-769e9fc592cf
	github.com/labstack/echo/v4 v4.6.1
	github.com/spf13/viper v1.3.1
	go.mongodb.org/mongo-driver v1.7.3
	gorm.io/gorm v1.22.4
)

//replace (
//	github.com/go-spring/spring-base => ../../spring/spring-base
//	github.com/go-spring/spring-core => ../../spring/spring-core
//	github.com/go-spring/spring-echo => ../../spring/spring-echo
//	github.com/go-spring/spring-go-redis => ../../spring/spring-go-redis
//	github.com/go-spring/starter-echo => ../../starter/starter-echo
//	github.com/go-spring/starter-go-redis => ../../starter/starter-go-redis
//	github.com/go-spring/starter-gorm => ../../starter/starter-gorm
//)
