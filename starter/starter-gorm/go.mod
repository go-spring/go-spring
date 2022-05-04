module github.com/go-spring/starter-gorm

go 1.14

require (
	github.com/DATA-DOG/go-sqlmock v1.5.0
	github.com/go-spring/spring-base v1.1.0-rc3.0.20220504021136-8ace13a580ab
	github.com/go-spring/spring-core v1.1.0-rc3.0.20220504021535-bb85dfe5bd69
	gorm.io/driver/mysql v1.2.1
	gorm.io/gorm v1.22.4
)

//replace (
//	github.com/go-spring/spring-base => ../../spring/spring-base
//	github.com/go-spring/spring-core => ../../spring/spring-core
//)
