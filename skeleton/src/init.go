package src

import (
	_ "GS_PROJECT_MODULE/src/app/grpcsvr"
	_ "GS_PROJECT_MODULE/src/app/httpsvr"
	_ "GS_PROJECT_MODULE/src/app/thriftsvr"

	_ "github.com/go-spring/starter-go-redis"
	_ "github.com/go-spring/starter-gorm-mysql"
	_ "github.com/go-spring/starter-redigo"
)

// 本文件通过 _ 空白导入自动配置的模块
// This file automatically configures modules via blank (_) imports.
