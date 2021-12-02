# starter-grpc

## Install

```
go get github.com/go-spring/starter-grpc@v1.1.0-rc2 
```

## Import

```
import "github.com/go-spring/starter-grpc/server"
```

## Example

```
package main

import (
	"context"
	"fmt"
	"log"

	pb "github.com/go-spring/examples/spring-boot-grpc/helloworld"
	"github.com/go-spring/spring-core/grpc"
	"github.com/go-spring/spring-core/gs"
	
	_ "github.com/go-spring/starter-grpc/server"
)

func init() {
	gs.Object(new(GreeterServer)).Init(func(s *GreeterServer) {
		gs.GrpcServer("helloworld.Greeter", &grpc.Server{
			Register: pb.RegisterGreeterServer,
			Service:  s,
		})
	})
}

type GreeterServer struct {
	AppName string `value:"${spring.application.name}"`
}

func (s *GreeterServer) SayHello(ctx context.Context, in *pb.HelloRequest) (*pb.HelloReply, error) {
	log.Printf("Received: %v", in.GetName())
	return &pb.HelloReply{Message: "Hello " + in.GetName() + " from " + s.AppName}, nil
}

func main() {
	gs.Property("spring.application.name", "GreeterServer")
	gs.Property("grpc.server.port", 50051)
	fmt.Println("application exit: ", gs.Run())
}
```