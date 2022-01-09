module github.com/go-spring/examples/spring-boot-demo

go 1.14

require (
	github.com/DATA-DOG/go-sqlmock v1.5.0
	github.com/go-spring/spring-base v1.1.0-rc2.0.20220108065257-1c285a12bc84
	github.com/go-spring/spring-core v1.1.0-rc2.0.20220109004946-9b2ba5cad23d
	github.com/go-spring/starter-echo v1.1.0-rc2.0.20220108073115-4928aa3f69b8
	github.com/go-spring/starter-go-redis v1.1.0-rc2.0.20220109005414-8ce9e0c354d0
	github.com/go-spring/starter-gorm v1.1.0-rc2.0.20220109005424-d2160c9a2366
	github.com/labstack/echo/v4 v4.6.1
	github.com/spf13/viper v1.3.1
	go.mongodb.org/mongo-driver v1.7.3
	gorm.io/driver/mysql v1.2.1
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
