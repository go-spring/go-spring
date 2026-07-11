// Package user also hosts the per-protocol controller adapters, distinguished
// by file suffix (user_controller_<proto>.go). This file is the gRPC adapter;
// it consumes the generated pb request/response types directly and delegates to
// the shared user application service.
package user

import (
	"context"

	"GS_PROJECT_MODULE/idl-domain/grpc/pb"
	"GS_PROJECT_MODULE/internal-domain/api/controller/user/converter"
	"GS_PROJECT_MODULE/internal-domain/application/user"
)

// GrpcUserController adapts gRPC user rpcs to the user application service.
type GrpcUserController struct {
	UserService *user.UserService `autowire:""`
}

// RegisterUser converts the pb request to a DTO, delegates to UserService, and
// wraps the result in a UserResp envelope.
func (c *GrpcUserController) RegisterUser(ctx context.Context, req *pb.RegisterUserReq) (*pb.UserResp, error) {
	u, err := c.UserService.RegisterUser(converter.FromGrpcRegisterUserReq(req))
	if err != nil {
		return &pb.UserResp{Code: 1, Message: err.Error()}, nil
	}
	return &pb.UserResp{Code: 0, Message: "ok", Data: converter.ToGrpcUser(u, req.GetEmail())}, nil
}

// UpgradeUser is a stub that mirrors the HTTP surface; a real implementation
// would call an upgrade path on the user application service.
func (c *GrpcUserController) UpgradeUser(ctx context.Context, req *pb.UpgradeUserReq) (*pb.UserResp, error) {
	// TODO: delegate to c.UserService once an upgrade path exists.
	_ = req
	return &pb.UserResp{Code: 0, Message: "ok"}, nil
}
