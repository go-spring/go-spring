module github.com/go-spring/examples/spring-boot-grpc

go 1.12

require (
	github.com/go-spring/spring-core v1.0.6-0.20210728141553-40e392cb8653
	github.com/go-spring/starter-gin v1.0.6-0.20210726123833-3624709a993f
	github.com/go-spring/starter-grpc v1.0.6-0.20210728142613-303f7199eb9f
	github.com/go-spring/starter-web v1.0.6-0.20210728142613-daf2d73d692f // indirect
	github.com/golang/protobuf v1.3.3
	google.golang.org/grpc v1.31.0
)

//replace (
//	github.com/go-spring/spring-core => ../../spring/spring-core
//	github.com/go-spring/starter-web => ../../starter/starter-web
//	github.com/go-spring/starter-grpc => ../../starter/starter-grpc
//)
