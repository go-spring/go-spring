module github.com/go-spring/starter-grpc

go 1.12

require (
	github.com/go-spring/spring-core v1.0.5
	github.com/go-spring/spring-stl v0.0.0-20210724145437-4e7cb5d3e0ce
	github.com/go-spring/starter-core v0.0.0-20210215012223-32c9b94871eb
	google.golang.org/grpc v1.31.0
)

replace (
	github.com/go-spring/spring-core => ../../spring/spring-core
	github.com/go-spring/spring-stl => ../../spring/spring-stl
	github.com/go-spring/starter-core => ../starter-core
)
