# starter-grpc

## Install

```
go get github.com/go-spring/starter-grpc@v1.1.0-rc2 
```

## Import

```
import "github.com/go-spring/starter-grpc/client"
```

## Example

```
package main

import (
	"fmt"

	pb "github.com/go-spring/examples/spring-boot-grpc/helloworld"
	"github.com/go-spring/spring-core/gs"
	"github.com/go-spring/spring-core/web"
	_ "github.com/go-spring/starter-gin"
	
	_ "github.com/go-spring/starter-grpc/client"
)

const (
	defaultName = "world"
)

func init() {
	gs.Provide(new(GreeterClientController)).Init(func(c *GreeterClientController) {
		gs.GetMapping("/", c.index)
	})
}

type GreeterClientController struct {
	GreeterClient pb.GreeterClient `autowire:""`
}

func (c *GreeterClientController) index(ctx web.Context) {
	r, err := c.GreeterClient.SayHello(ctx.Request().Context(), &pb.HelloRequest{Name: defaultName})
	web.ERROR.Panic(err).When(err != nil)
	ctx.String("Greeting: " + r.GetMessage())
}

func init() {
	gs.GrpcClient(pb.NewGreeterClient, "greeter-client")
}

func main() {
	gs.Property("grpc.endpoint.greeter-client.address", "127.0.0.1:50051")
	gs.Property("spring.application.name", "GreeterClient")
	fmt.Println("application exit: ", gs.Run())
}
```