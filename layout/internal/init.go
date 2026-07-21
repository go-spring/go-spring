// Package domain is the root package for the domain-driven layer.
// It imports server and job packages to trigger auto-wiring of all controllers and services.
package domain

import (
	_ "GS_PROJECT_MODULE/internal/api/job"
	_ "GS_PROJECT_MODULE/internal/api/server/dubbosvr"
	_ "GS_PROJECT_MODULE/internal/api/server/echosvr"
	_ "GS_PROJECT_MODULE/internal/api/server/ginsvr"
	_ "GS_PROJECT_MODULE/internal/api/server/goframe-grpcsvr"
	_ "GS_PROJECT_MODULE/internal/api/server/goframe-httpsvr"
	_ "GS_PROJECT_MODULE/internal/api/server/goframe-tcpsvr"
	_ "GS_PROJECT_MODULE/internal/api/server/goframe-wssvr"
	_ "GS_PROJECT_MODULE/internal/api/server/gozero-restsvr"
	_ "GS_PROJECT_MODULE/internal/api/server/grpcsvr"
	_ "GS_PROJECT_MODULE/internal/api/server/hertzsvr"
	_ "GS_PROJECT_MODULE/internal/api/server/httpsvr"
	_ "GS_PROJECT_MODULE/internal/api/server/kitex-grpcsvr"
	_ "GS_PROJECT_MODULE/internal/api/server/kitex-thriftsvr"
	_ "GS_PROJECT_MODULE/internal/api/server/kratos-grpcsvr"
	_ "GS_PROJECT_MODULE/internal/api/server/kratos-httpsvr"
	_ "GS_PROJECT_MODULE/internal/api/server/kratos-wssvr"
	_ "GS_PROJECT_MODULE/internal/api/server/mqsvr"
	_ "GS_PROJECT_MODULE/internal/api/server/thriftsvr"
	_ "GS_PROJECT_MODULE/internal/api/server/trpcsvr"
	_ "GS_PROJECT_MODULE/internal/api/server/wssvr"

	_ "go-spring.org/starter-echo"
	_ "go-spring.org/starter-gin"
	_ "go-spring.org/starter-go-redis"
	_ "go-spring.org/starter-gorm-mysql"
	_ "go-spring.org/starter-hertz"
)
