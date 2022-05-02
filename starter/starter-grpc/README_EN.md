# starter-grpc

[中文](README.md)

[仅发布] 该项目仅为最终发布，不要向该项目直接提交代码，开发请关注 [go-spring](https://github.com/go-spring/go-spring) 项目。

## Installation

### Prerequisites

- Go >= 1.12

### Using go get

```
go get github.com/go-spring/starter-grpc@v1.1.0-rc2 
```

## Quick Start

### gRPC Server

```
import "github.com/go-spring/starter-grpc/server"
```

`main.go`

```
package main

import (
	"context"
	"fmt"
	"log"

	"github.com/go-spring/spring-core/grpc"
	"github.com/go-spring/spring-core/gs"
	pb "github.com/go-spring/starter-grpc/example/helloworld"

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
	fmt.Println("application exit: ", gs.Web(false).Run())
}
```

### gRPC Client

```
import "github.com/go-spring/starter-grpc/client"
```

`main.go`

```
package main

import (
	"context"
	"fmt"

	"github.com/go-spring/spring-core/gs"
	"github.com/go-spring/spring-core/web"
	pb "github.com/go-spring/starter-grpc/example/helloworld"

	_ "github.com/go-spring/starter-grpc/client"
)

const (
	defaultName = "world"
)

func init() {
	gs.GrpcClient(pb.NewGreeterClient, "greeter")
	gs.Object(&runner{}).Export((*gs.AppRunner)(nil))
}

type runner struct {
	GreeterClient pb.GreeterClient `autowire:""`
}

func (r *runner) Run(ctx gs.Context) {
	resp, err := r.GreeterClient.SayHello(context.TODO(), &pb.HelloRequest{Name: defaultName})
	web.ERROR.Panic(err).When(err != nil)
	fmt.Println("Greeting: " + resp.GetMessage())
	go gs.ShutDown()
}

func main() {
	gs.Property("grpc.endpoint.greeter.address", "127.0.0.1:50051")
	gs.Property("spring.application.name", "GreeterClient")
	fmt.Println("application exit: ", gs.Web(false).Run())
}
```

## Configuration
