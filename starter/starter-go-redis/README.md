# starter-go-redis

[仅发布] 该项目仅为最终发布，不要向该项目直接提交代码，开发请关注 [go-spring](https://github.com/go-spring/go-spring) 项目。

- [Installation](#installation)
- [Quick Start](#quick start)
- [Customization](#customization)

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
	"errors"
	"fmt"

	"github.com/go-spring/spring-base/util"
	"github.com/go-spring/spring-core/gs"
	"github.com/go-spring/spring-core/redis"
	_ "github.com/go-spring/starter-go-redis"
)

type runner struct {
	Client redis.Client `autowire:""`
}

func (r *runner) Run(ctx gs.Context) {

	_, err := r.Client.Get(ctx.Context(), "nonexisting")
	if err != redis.ErrNil {
		panic(errors.New("should be redis.ErrNil"))
	}

	_, err = r.Client.Set(ctx.Context(), "mykey", "Hello")
	util.Panic(err).When(err != nil)

	v, err := r.Client.Get(ctx.Context(), "mykey")
	util.Panic(err).When(err != nil)
	if v != "Hello" {
		panic(errors.New("should be \"Hello\""))
	}

	fmt.Printf("GET mykey=%q\n", v)
	go gs.ShutDown()
}

func main() {
	gs.Object(&runner{}).Export((*gs.AppRunner)(nil))
	fmt.Printf("program exited %v\n", gs.Web(false).Run())
}
```