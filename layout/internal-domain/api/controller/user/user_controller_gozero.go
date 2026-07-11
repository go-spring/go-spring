// Package user also hosts the per-protocol controller adapters, distinguished by
// file suffix (user_controller_<proto>.go). This file is the go-zero adapter; see
// user_controller.go for the HTTP baseline. All adapters delegate to the same
// user application service, so every protocol converges on one core.
package user

import (
	"context"

	types "GS_PROJECT_MODULE/idl-domain/gozero/types"
	"GS_PROJECT_MODULE/internal-domain/api/controller/user/converter"
	"GS_PROJECT_MODULE/internal-domain/application/user"
)

// GozeroUserController adapts go-zero-native user calls to the user application
// service. Method signatures match the generated GS_PROJECT_NAMELogic interface
// so the composed server controller can satisfy it through embedding.
type GozeroUserController struct {
	UserService *user.UserService `autowire:""`
}

// RegisterUser converts the go-zero request into an application DTO, delegates
// to UserService, and maps the result back to the go-zero response envelope.
func (c *GozeroUserController) RegisterUser(ctx context.Context, req *types.RegisterUserReq) (*types.UserResp, error) {
	u, err := c.UserService.RegisterUser(converter.FromGozeroRegisterUserReq(req))
	if err != nil {
		return &types.UserResp{Code: 1, Message: err.Error()}, nil
	}
	return &types.UserResp{Data: converter.ToGozeroUser(u, req.Email)}, nil
}

// UpgradeUser is a stub that returns an empty success envelope. The
// application layer currently has no upgrade-privileges method; wire it in
// once the domain grows one.
func (c *GozeroUserController) UpgradeUser(ctx context.Context, req *types.UpgradeUserReq) (*types.UserResp, error) {
	return &types.UserResp{}, nil
}
