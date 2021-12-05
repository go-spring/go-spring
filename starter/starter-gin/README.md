# starter-gin

[仅发布] 该项目仅为最终发布，开发请关注 [go-spring](https://github.com/go-spring/go-spring) 项目。

## Install

```
go get github.com/go-spring/starter-gin@v1.1.0-rc2 
```

## Import

```
import "github.com/go-spring/starter-gin"
```

## Example

```
package main

import (
	"fmt"

	"github.com/go-spring/spring-core/gs"
	"github.com/go-spring/spring-core/web"
	_ "github.com/go-spring/starter-gin"
)

func init() {
	gs.Object(new(Controller)).Init(func(c *Controller) {
		gs.GetMapping("/", c.Hello)
	})
}

type Controller struct {
	GOPATH string `value:"${GOPATH}"`
}

func (c *Controller) Hello(ctx web.Context) {
	ctx.String("%s - hello world!", c.GOPATH)
}

func main() {
	fmt.Println(gs.Run())
}
```