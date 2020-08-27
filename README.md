# spring-boot

该包提供了一种基于 spring-core 的启动框架。

```
package main

import (
	"github.com/go-spring/go-spring/spring-boot"
	_ "github.com/go-spring/go-spring/starter-echo"
)

func init() {
	SpringBoot.RegisterBean(new(Echo)).Init(func(e *Echo) {
		SpringBoot.GetBinding("/", e.Call)
	})
}

type Echo struct {
	GoPath string `value:"${GOPATH}"`
}

func (e *Echo) Call() string {
	return e.GoPath
}

func main() {
	SpringBoot.RunApplication()
}
```