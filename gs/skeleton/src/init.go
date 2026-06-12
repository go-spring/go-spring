package src

import (
	_ "GS_PROJECT_MODULE/src/app/grpcsvr"
	_ "GS_PROJECT_MODULE/src/app/httpsvr"
	_ "GS_PROJECT_MODULE/src/app/thriftsvr"

	_ "go-spring.org/starter-go-redis"
	_ "go-spring.org/starter-gorm-mysql"
	_ "go-spring.org/starter-redigo"
)

// 本文件通过 _ 空白导入自动配置的模块
// This file automatically configures modules via blank (_) imports.
