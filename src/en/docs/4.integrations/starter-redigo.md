# starter-redigo - Redis Integration (redigo)

> redigo is another popular Redis Go client.

## Installation

```go
import _ "github.com/go-spring/starter-redigo"
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

Inject `redigo.Conn` or `redigo.Pool` where it is needed:

```go
package app

import (
	"github.com/gomodule/redigo/redis"
)

type CacheService struct {
	pool *redis.Pool `autowire:""`
}
```
