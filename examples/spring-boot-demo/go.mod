module github.com/go-spring/examples/spring-boot-demo

go 1.14

require (
	github.com/DATA-DOG/go-sqlmock v1.5.0
	github.com/dgrijalva/jwt-go v3.2.0+incompatible // indirect
	github.com/go-spring/spring-base v1.1.0-rc1.0.20211113013245-ace7b4d73418
	github.com/go-spring/spring-core v1.1.0-rc1.0.20211113013626-adb083a077e7
	github.com/go-spring/starter-echo v1.1.0-rc1.0.20211113023731-1df11ce6f9c6
	github.com/go-spring/starter-go-redis v1.1.0-rc1.0.20211113023805-62a428f12c80
	github.com/go-spring/starter-gorm v1.1.0-rc1.0.20211113024007-ee7df83b094c
	github.com/jinzhu/gorm v1.9.16
	github.com/labstack/echo v3.3.10+incompatible
	github.com/spf13/viper v1.3.1
	go.mongodb.org/mongo-driver v1.7.3
)

//replace (
//	github.com/go-spring/spring-base => ../../spring/spring-base
//	github.com/go-spring/spring-core => ../../spring/spring-core
//	github.com/go-spring/spring-echo => ../../spring/spring-echo
//	github.com/go-spring/spring-go-redis => ../../spring/spring-go-redis
//	github.com/go-spring/starter-web => ../../starter/starter-web
//)
