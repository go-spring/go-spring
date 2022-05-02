# starter-go-mongo

[English](README_EN.md)

[仅发布] 该项目仅为最终发布，不要向该项目直接提交代码，开发请关注 [go-spring](https://github.com/go-spring/go-spring) 项目。

## Installation

### Prerequisites

- Go >= 1.12

### Using go get

```
go get github.com/go-spring/starter-go-mongo@v1.1.0-rc2 
```

## Quick Start

```
import "github.com/go-spring/starter-go-mongo"
```

`main.go`

```
package main

import (
	"context"
	"fmt"

	"github.com/go-spring/spring-base/util"
	"github.com/go-spring/spring-core/gs"
	_ "github.com/go-spring/starter-go-mongo"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
)

type runner struct {
	Client *mongo.Client `autowire:""`
}

func (r *runner) Run(ctx gs.Context) {

	collection := r.Client.Database("baz").Collection("qux")
	_, err := collection.InsertOne(context.Background(), bson.M{"hello": "world", "Foo": "1"})
	util.Panic(err).When(err != nil)

	ret := struct{ Foo string }{}
	filter := bson.D{{"hello", "world"}}
	err = collection.FindOne(context.Background(), filter).Decode(&ret)
	util.Panic(err).When(err != nil)
	fmt.Println(ret)

	go gs.ShutDown()
}

func main() {
	gs.Object(&runner{}).Export((*gs.AppRunner)(nil))
	fmt.Printf("program exited %v\n", gs.Web(false).Run())
}
```

## Configuration