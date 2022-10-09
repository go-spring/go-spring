module github.com/go-spring/starter-gorm

go 1.14

require (
	github.com/DATA-DOG/go-sqlmock v1.5.0
	github.com/go-spring/spring-base v1.1.3-0.20221009074117-5fc71d4a6063
	github.com/go-spring/spring-core v1.1.3-0.20221009074341-cea99bf58de3
	gorm.io/driver/mysql v1.2.1
	gorm.io/gorm v1.22.4
)

//replace (
//	github.com/go-spring/spring-base => ../../spring/spring-base
//	github.com/go-spring/spring-core => ../../spring/spring-core
//)
