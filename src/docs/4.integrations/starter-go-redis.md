# starter-go-redis - Redis 集成 (go-redis)

> go-redis 是 Redis 生态最流行的 Go 客户端。

## 安装

```go
import _ "github.com/go-spring/starter-go-redis"
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

在需要使用的地方注入 `*redis.Client`：

```go
package app

import (
	"context"
	"github.com/redis/go-redis/v9"
)

type CacheService struct {
	rdb *redis.Client `autowire:""`
}

func (s *CacheService) Get(ctx context.Context, key string) (string, error) {
	return s.rdb.Get(ctx, key).Result()
}
```
