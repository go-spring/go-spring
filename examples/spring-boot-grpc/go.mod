module github.com/go-spring/examples/spring-boot-grpc

go 1.14

require (
	github.com/go-spring/spring-core v1.0.6-0.20211022224649-f0f6fffd8bc2
	github.com/go-spring/starter-gin v1.1.0-alpha.0.20211023011805-8215986fdbc1
	github.com/go-spring/starter-grpc v1.1.0-alpha.0.20211023010928-560d08e73b00
	github.com/golang/protobuf v1.4.3
	google.golang.org/grpc v1.41.0
)

//replace (
//	github.com/go-spring/spring-core => ../../spring/spring-core
//	github.com/go-spring/starter-web => ../../starter/starter-web
//	github.com/go-spring/starter-grpc => ../../starter/starter-grpc
//)
