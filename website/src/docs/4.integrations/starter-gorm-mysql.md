# starter-gorm-mysql - MySQL GORM 集成

> GORM 是 Go 生态最流行的 ORM 库。

## 安装

```go
import _ "github.com/go-spring/starter-gorm-mysql"
```

## 配置

```properties
# DSN 连接字符串 (必填)
gorm.mysql.dsn=user:password@tcp(127.0.0.1:3306)/dbname?charset=utf8mb4&parseTime=True&loc=Local

# 是否开启 GORM debug 日志 (默认 false)
gorm.mysql.debug=true
```

## 使用

在需要使用的地方注入 `*gorm.DB`：

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
