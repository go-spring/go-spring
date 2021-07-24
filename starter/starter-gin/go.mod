module github.com/go-spring/starter-gin

go 1.12

require (
	github.com/go-spring/spring-core v1.0.6-0.20201217060132-0c182ff5a770
	github.com/go-spring/spring-gin v1.0.6-0.20201215104813-bb32b4fe5dbb
	github.com/go-spring/starter-core v0.0.0-20210215012223-32c9b94871eb
	github.com/go-spring/starter-web v1.0.6-0.20201222111800-2895789c2981
	github.com/go-spring/spring-stl v0.0.0-20210724145437-4e7cb5d3e0ce // indirect
)

replace (
	github.com/go-spring/spring-stl => ../../spring/spring-stl
	github.com/go-spring/spring-core => ../../spring/spring-core
	github.com/go-spring/spring-gin => ../../spring/spring-gin
	github.com/go-spring/starter-core => ../starter-core
	github.com/go-spring/starter-web => ../starter-web
)
