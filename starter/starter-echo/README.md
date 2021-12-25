# starter-echo

[仅发布] 该项目仅为最终发布，不要向该项目直接提交代码，开发请关注 [go-spring](https://github.com/go-spring/go-spring) 项目。

- [Installation](#installation)
- [Quick Start](#quick start)
- [Customization](#customization)

## Install

### Prerequisites

- Go

### Using go get

```
go get github.com/go-spring/starter-echo@v1.1.0-rc2 
```

### Import

```
import _ "github.com/go-spring/starter-echo"
```

## Quick Start

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

上面这段代码使用默认配置启动了一个 8080 端口的 web 服务器，
Controller 内使用 `value` tag 为其注入了一个环境变量 GOPATH，
gs.GetMapping 注册了一个路由为 "/" 的 Get 方法，Controller.Hello 为其响应函数。

```
➜ curl http://localhost:8080/
/Users/didi/work - hello world!
```

## Customization

### Port

为 [Quick Start](#quick start) 章节的示例添加配置文件 `config/application.properties`，内容如下，

```
web.server.port=8000
```

然后通过启动日志可以看到服务器的端口已经变成了 8000 。

```
➜ curl http://localhost:8000/
/Users/didi/work - hello world!
```