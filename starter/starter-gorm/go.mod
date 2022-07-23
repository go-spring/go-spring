module github.com/go-spring/starter-gorm

go 1.14

require (
	github.com/DATA-DOG/go-sqlmock v1.5.0
	github.com/go-spring/spring-base v1.1.0-rc4.0.20220723014310-0954983fa7c4
	github.com/go-spring/spring-core v1.1.0-rc4.0.20220723014617-b854e072484a
	gorm.io/driver/mysql v1.2.1
	gorm.io/gorm v1.22.4
)

//replace (
//	github.com/go-spring/spring-base => ../../spring/spring-base
//	github.com/go-spring/spring-core => ../../spring/spring-core
//)
