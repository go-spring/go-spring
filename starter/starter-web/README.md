# starter-web

## Install

```
go get github.com/go-spring/starter-web@v1.1.0-rc2 
```

## Import

```
import _ "github.com/go-spring/starter-web"
```

## Example

```
package main

import (
  "fmt"

  "github.com/go-spring/spring-core/gs"
  "github.com/go-spring/spring-core/web"
  "github.com/go-spring/spring-gin"
  
  _ "github.com/go-spring/starter-web"
)

func init() {
  gs.Object(SpringGin.NewContainer(web.ContainerConfig{
    Port:     8080,
    BasePath: "/",
  })).Export((*web.Container)(nil))
}

func main() {
  gs.GetMapping("/", func(ctx web.Context) {
    ctx.String("Hello World!")
  })
  fmt.Println(gs.Run())
}
```