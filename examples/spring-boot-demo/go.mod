module github.com/go-spring/examples/spring-boot-demo

go 1.14

require (
	github.com/DATA-DOG/go-sqlmock v1.5.0
	github.com/dgrijalva/jwt-go v3.2.0+incompatible // indirect
	github.com/go-spring/spring-base v1.1.0-rc1
	github.com/go-spring/spring-core v1.1.0-rc1
	github.com/go-spring/starter-echo v1.1.0-rc1
	github.com/go-spring/starter-go-redis v1.1.0-rc1
	github.com/go-spring/starter-gorm v1.1.0-rc1
	github.com/jinzhu/gorm v1.9.16
	github.com/labstack/echo v3.3.10+incompatible
	github.com/spf13/viper v1.3.1
	go.mongodb.org/mongo-driver v1.7.3
)

//replace (
//	github.com/go-spring/spring-core => ../../spring/spring-core
//	github.com/go-spring/starter-web => ../../starter/starter-web
//)
