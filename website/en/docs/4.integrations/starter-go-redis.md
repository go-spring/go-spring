# starter-go-redis - Redis Integration (go-redis)

> go-redis is the most popular Go client in the Redis ecosystem.

## Installation

```go
import _ "go-spring.org/starter-go-redis"
```

## Configuration

```properties
# Redis server address (required)
redis.addr=localhost:6379

# Password (optional, empty by default)
redis.password=

# Database number (default: 0)
redis.db=0
```

## Usage

Inject `*redis.Client` where it is needed:

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
