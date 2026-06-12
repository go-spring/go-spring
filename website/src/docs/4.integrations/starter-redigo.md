# starter-redigo - Redis 集成 (redigo)

> redigo 是另一个流行的 Redis Go 客户端。

## 安装

```go
import _ "github.com/go-spring/starter-redigo"
```

## 配置

```properties
# Redis 服务器地址 (必填)
redis.addr=localhost:6379

# 密码 (可选，默认为空)
redis.password=

# 数据库编号 (默认 0)
redis.db=0
```

## 使用

在需要使用的地方注入 `redigo.Conn` 或者 `redigo.Pool`：

```go
package app

import (
	"github.com/gomodule/redigo/redis"
)

type CacheService struct {
	pool *redis.Pool `autowire:""`
}
```
