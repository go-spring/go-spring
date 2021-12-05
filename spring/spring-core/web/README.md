# web

为社区优秀的 Web 服务器提供一个抽象层，使得底层可以灵活切换。

## Install

```
go get github.com/go-spring/spring-core@v1.1.0-rc2 
```

## Import

```
import "github.com/go-spring/spring-core/web"
```

## Example

### 普通路由

```
package main

import (
	"fmt"

	"github.com/go-spring/spring-core/gs"
	"github.com/go-spring/spring-core/web"
	_ "github.com/go-spring/starter-echo"
)

func main() {
	gs.GetMapping("/a/b/c", func(ctx web.Context) {
		ctx.String("OK")
	})
	fmt.Println(gs.Run())
}
```

```
➜ curl http://127.0.0.1:8080/a/b/c
OK
```

### java 风格路由

```
package main

import (
	"fmt"

	"github.com/go-spring/spring-core/gs"
	"github.com/go-spring/spring-core/web"
	_ "github.com/go-spring/starter-echo"
)

func main() {
	gs.GetMapping("/:a/b/:c/{*:d}", func(ctx web.Context) {
		ctx.String("a=%s b=%s *=%s\n", ctx.PathParam("a"), ctx.PathParam("c"), ctx.PathParam("*"))
		ctx.String("a=%s b=%s *=%s\n", ctx.PathParam("a"), ctx.PathParam("c"), ctx.PathParam("d"))
	})
	fmt.Println(gs.Run())
}
```

```
➜ curl http://127.0.0.1:8080/a/b/c/d
a=a b=c *=d
a=a b=c *=d
```

### echo 风格路由

```
package main

import (
	"fmt"

	"github.com/go-spring/spring-core/gs"
	"github.com/go-spring/spring-core/web"
	_ "github.com/go-spring/starter-echo"
)

func main() {
	gs.GetMapping("/:a/b/:c/*", func(ctx web.Context) {
		ctx.String("a=%s c=%s *=%s", ctx.PathParam("a"), ctx.PathParam("c"), ctx.PathParam("*"))
	})
	fmt.Println(gs.Run())
}
```

```
➜ curl http://127.0.0.1:8080/a/b/c/d
a=a c=c *=d
```

### gin 风格路由

```
package main

import (
	"fmt"

	"github.com/go-spring/spring-core/gs"
	"github.com/go-spring/spring-core/web"
	_ "github.com/go-spring/starter-echo"
)

func main() {
	gs.GetMapping("/:a/b/:c/*d", func(ctx web.Context) {
		ctx.String("a=%s b=%s *=%s\n", ctx.PathParam("a"), ctx.PathParam("c"), ctx.PathParam("*"))
		ctx.String("a=%s b=%s *=%s\n", ctx.PathParam("a"), ctx.PathParam("c"), ctx.PathParam("d"))
	})
	fmt.Println(gs.Run())
}
```

```
➜ curl http://127.0.0.1:8080/a/b/c/d
a=a b=c *=d
a=a b=c *=d
```

### 文件服务器

```
package main

import (
	"fmt"
	"net/http"

	"github.com/go-spring/spring-core/gs"
	"github.com/go-spring/spring-core/web"
	_ "github.com/go-spring/starter-echo"
)

func main() {
	gs.HandleGet("/public/*", web.WrapH(http.StripPrefix("/public/", http.FileServer(http.Dir("public")))))
	fmt.Println(gs.Run())
}
```

然后在项目下创建一个 public 目录，里面创建一个内容为 hello world! 的 a.txt 文件。

```
➜ curl http://127.0.0.1:8080/public/a.txt
hello world!
```

### BIND 模式

```
package main

import (
	"context"
	"fmt"

	"github.com/go-spring/spring-core/gs"
	"github.com/go-spring/spring-core/web"
	_ "github.com/go-spring/starter-echo"
)

type HelloReq struct {
	Name string `form:"name"`
}

type HelloResp struct {
	Body string `json:"body"`
}

func main() {
	gs.GetBinding("/hello", func(ctx context.Context, req *HelloReq) *web.RpcResult {
		return web.SUCCESS.Data(&HelloResp{Body: "hello " + req.Name + "!"})
	})
	fmt.Println(gs.Run())
}
```

```
➜ curl 'http://127.0.0.1:8080/hello?name=lvan100' 
{"code":200,"msg":"SUCCESS","data":{"body":"hello lvan100!"}}
```

### 中间件

#### Basic Auth

```
package main

import (
	"fmt"

	"github.com/go-spring/spring-core/gs"
	"github.com/go-spring/spring-core/web"
	"github.com/go-spring/spring-echo"
	_ "github.com/go-spring/starter-echo"
	"github.com/labstack/echo"
	"github.com/labstack/echo/middleware"
)

func main() {

	gs.Provide(func( /* 可以通过配置将用户名密码传进来 */ ) web.Filter {
		m := middleware.BasicAuth(func(u string, p string, e echo.Context) (bool, error) {
			if u == "lvan100" && p == "123456" {
				return true, nil
			}
			return false, nil
		})
		return SpringEcho.Filter(m)
	})

	gs.GetMapping("/hello", func(ctx web.Context) {
		ctx.String("hello %s!", ctx.QueryParam("name"))
	})

	fmt.Println(gs.Run())
}
```

```
➜ curl 'http://127.0.0.1:8080/hello?name=lvan100'
Unauthorized
➜ curl 'http://127.0.0.1:8080/hello?name=lvan100' -H'Authorization: Basic bHZhbjEwMDoxMjM0NTY='
{"code":200,"msg":"SUCCESS","data":{"body":"hello lvan100!"}}
```