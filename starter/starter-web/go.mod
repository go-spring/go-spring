module github.com/go-spring/starter-web

go 1.12

require (
	github.com/go-spring/spring-core v1.0.6-0.20201217060132-0c182ff5a770
	github.com/go-spring/spring-stl v0.0.0-20210724145437-4e7cb5d3e0ce
)

replace (
	github.com/go-spring/spring-core => ../../spring/spring-core
	github.com/go-spring/spring-stl => ../../spring/spring-stl
)
