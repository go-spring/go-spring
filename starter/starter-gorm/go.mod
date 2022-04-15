module github.com/go-spring/starter-gorm

go 1.14

require (
	github.com/go-spring/spring-base v1.1.0-rc3
	github.com/go-spring/spring-core v1.1.0-rc3
	gorm.io/driver/mysql v1.2.1
	gorm.io/gorm v1.22.4
)

replace (
	github.com/go-spring/spring-base => ../../spring/spring-base
	github.com/go-spring/spring-core => ../../spring/spring-core
)
