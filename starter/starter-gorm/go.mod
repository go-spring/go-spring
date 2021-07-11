module github.com/go-spring/starter-gorm

go 1.12

require (
	github.com/go-spring/spring-core v1.0.5
	github.com/go-spring/starter-core v0.0.0-20210215012223-32c9b94871eb
	github.com/jinzhu/gorm v1.9.15
)

replace (
	github.com/go-spring/spring-core => ../../spring/spring-core
	github.com/go-spring/starter-core => ../starter-core
)