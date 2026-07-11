// Package domain is the root package for the domain-driven layer.
// It imports server and job packages to trigger auto-wiring of all controllers and services.
package domain

import (
	_ "GS_PROJECT_MODULE/internal/api/job"
	_ "GS_PROJECT_MODULE/internal/api/server/dubbosvr"
	_ "GS_PROJECT_MODULE/internal/api/server/gozerosvr"
	_ "GS_PROJECT_MODULE/internal/api/server/grpcsvr"
	_ "GS_PROJECT_MODULE/internal/api/server/httpsvr"
	_ "GS_PROJECT_MODULE/internal/api/server/kitexsvr"
	_ "GS_PROJECT_MODULE/internal/api/server/mqsvr"
	_ "GS_PROJECT_MODULE/internal/api/server/thriftsvr"
	_ "GS_PROJECT_MODULE/internal/api/server/wssvr"

	_ "go-spring.org/starter-go-redis"
	_ "go-spring.org/starter-gorm-mysql"
)
