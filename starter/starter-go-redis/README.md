# starter-go-redis

## Install

```
go get github.com/go-spring/starter-go-redis@v1.1.0-rc2 
```

## Import

```
import "github.com/go-spring/starter-go-redis"
```

## Example

```
package main

import (
	"fmt"

	"github.com/go-spring/spring-base/util"
	"github.com/go-spring/spring-core/gs"
	"github.com/go-spring/spring-core/redis"
	_ "github.com/go-spring/starter-go-redis"
)

type runner struct {
	RedisClient redis.Client `autowire:""`
}

func (r *runner) Run(ctx gs.Context) {
	_, err := r.RedisClient.Set(ctx.Context(), "a", 1)
	util.Panic(err).When(err != nil)
	v, err := r.RedisClient.Get(ctx.Context(), "a")
	util.Panic(err).When(err != nil)
	fmt.Printf("get redis a=%v\n", v)
	go gs.ShutDown()
}

func main() {
	gs.Object(&runner{}).Export((*gs.AppRunner)(nil))
	fmt.Printf("program exited %v\n", gs.Run())
}
```