module github.com/go-spring/examples/spring-boot-grpc

go 1.14

require (
	github.com/go-spring/spring-core v1.0.6-0.20211001040940-f4fed6e6c943
	github.com/go-spring/starter-gin v1.1.0-alpha.0.20211002014739-f8bd61167c18
	github.com/go-spring/starter-grpc v1.1.0-alpha.0.20211002011045-88225be3ef61
	github.com/golang/protobuf v1.3.3
	google.golang.org/grpc v1.31.0
)

//replace (
//	github.com/go-spring/spring-core => ../../spring/spring-core
//	github.com/go-spring/starter-web => ../../starter/starter-web
//	github.com/go-spring/starter-grpc => ../../starter/starter-grpc
//)
