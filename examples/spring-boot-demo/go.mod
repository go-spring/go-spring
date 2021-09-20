module github.com/go-spring/examples/spring-boot-demo

go 1.14

require (
	github.com/DATA-DOG/go-sqlmock v1.4.1
	github.com/dgrijalva/jwt-go v3.2.0+incompatible // indirect
	github.com/elliotchance/redismock v1.5.3
	github.com/go-redis/redis v6.15.9+incompatible
	github.com/go-spring/spring-boost v0.0.0-20210919065342-b77a23a19833
	github.com/go-spring/spring-core v1.0.6-0.20210920010126-487455749735
	github.com/go-spring/starter-echo v1.1.0-alpha.0.20210919081451-5be75d54f046
	github.com/go-spring/starter-go-redis v1.1.0-alpha.0.20210919082232-b7a1a671a5ad
	github.com/go-spring/starter-gorm v1.1.0-alpha.0.20210919082328-d4be8c0ecdf4
	github.com/jinzhu/gorm v1.9.15
	github.com/labstack/echo v3.3.10+incompatible
	github.com/spf13/viper v1.3.1
	go.mongodb.org/mongo-driver v1.5.1
)

//replace (
//	github.com/go-spring/spring-core => ../../spring/spring-core
//)
