# starter-echo

[中文](README.md)

[仅发布] 该项目仅为最终发布，不要向该项目直接提交代码，开发请关注 [go-spring](https://github.com/go-spring/go-spring) 项目。

## Install

### Prerequisites

- Go >= 1.12

### Using go get

```
go get github.com/go-spring/starter-echo@v1.1.0-rc2 
```

## Quick Start

```
import _ "github.com/go-spring/starter-echo"
```

`main.go`

```
package main

import (
	"fmt"

	"github.com/go-spring/spring-core/gs"
	"github.com/go-spring/spring-core/web"
	_ "github.com/go-spring/starter-echo"
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

`config/application.properties`

```
web.server.port=8000
```

```
➜ curl http://localhost:8000/
/Users/didi/work - hello world!
```

## Customization
