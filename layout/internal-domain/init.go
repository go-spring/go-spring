// Package domain is the root package for the domain-driven layer.
// It imports server and job packages to trigger auto-wiring of all controllers and services.
package domain

import (
	_ "GS_PROJECT_MODULE/internal-domain/api/job"
	_ "GS_PROJECT_MODULE/internal-domain/api/server/grpcsvr"
	_ "GS_PROJECT_MODULE/internal-domain/api/server/httpsvr"
	_ "GS_PROJECT_MODULE/internal-domain/api/server/mqsvr"
	_ "GS_PROJECT_MODULE/internal-domain/api/server/thriftsvr"
	_ "GS_PROJECT_MODULE/internal-domain/api/server/wssvr"
	_ "GS_PROJECT_MODULE/internal-domain/infra/repo/order"
	_ "GS_PROJECT_MODULE/internal-domain/infra/repo/user"
)
