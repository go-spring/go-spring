# starter-gorm-mysql - MySQL GORM Integration

> GORM is the most popular ORM library in the Go ecosystem.

## Installation

```go
import _ "go-spring.org/starter-gorm-mysql"
```

## Configuration

```properties
# DSN connection string (required)
gorm.mysql.dsn=user:password@tcp(127.0.0.1:3306)/dbname?charset=utf8mb4&parseTime=True&loc=Local

# Whether to enable GORM debug logs (default: false)
gorm.mysql.debug=true
```

## Usage

Inject `*gorm.DB` where it is needed:

```go
package app

import (
	"gorm.io/gorm"
)

type UserService struct {
	db *gorm.DB `autowire:""`
}

func (s *UserService) GetUser(id int64) (*User, error) {
	var user User
	if err := s.db.Where("id = ?", id).First(&user).Error; err != nil {
		return nil, err
	}
	return &user, nil
}
```
