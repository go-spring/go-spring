# starter-gorm

[中文](README.md)

[仅发布] 该项目仅为最终发布，不要向该项目直接提交代码，开发请关注 [go-spring](https://github.com/go-spring/go-spring) 项目。

## Installation

### Prerequisites

- Go >= 1.12

### Using go get

```
go get github.com/go-spring/starter-gorm@v1.1.0-rc2 
```

## Quick Start

```
import "github.com/go-spring/starter-gorm/mysql"
```

`main.go`

```
package main

import (
	"fmt"

	"github.com/go-spring/spring-base/log"
	"github.com/go-spring/spring-core/gs"
	"gorm.io/gorm"

	_ "github.com/go-spring/starter-gorm/mysql"
)

type runner struct {
	DB *gorm.DB `autowire:""`
}

func (r *runner) Run(ctx gs.Context) {
	var engines []string
	r.DB.Raw("select engine from engines").Scan(&engines)
	log.Infof("got mysql engines %v", engines)
	go gs.ShutDown()
}

func main() {
	gs.Object(&runner{}).Export((*gs.AppRunner)(nil))
	fmt.Printf("program exited %v\n", gs.Web(false).Run())
}
```

`application.properties`

```
db.url=root:@/information_schema?charset=utf8&parseTime=True&loc=Local
```

## Customization

