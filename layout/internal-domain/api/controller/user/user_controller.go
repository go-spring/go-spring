// Package user is the controller layer for the user domain.
// It validates incoming requests, delegates to application services, and formats responses.
package user

import (
	"context"

	"GS_PROJECT_MODULE/idl-domain/http/proto"
	"GS_PROJECT_MODULE/internal-domain/api/controller/user/converter"
	"GS_PROJECT_MODULE/internal-domain/application/user"
)

// UserController implements user-related API handlers.
type UserController struct {
	UserService *user.UserService `autowire:""`
}

// RegisterUser creates a new user and returns the registration result.
func (c *UserController) RegisterUser(ctx context.Context, req *proto.RegisterUserReq) *proto.RegisterUserResp {
	user, err := c.UserService.RegisterUser(converter.FromRegisterUserReq(req))
	if err != nil {
		return &proto.RegisterUserResp{
			Errno:  proto.ErrCode_PARAM_ERROR,
			Errmsg: err.Error(),
		}
	}

	return &proto.RegisterUserResp{
		Errno:  proto.ErrCode_ErrOk,
		Errmsg: proto.ErrCode_name[proto.ErrCode_ErrOk],
		Data:   converter.ToProtoUser(user, req.Email),
	}
}

// UpgradeUser upgrades an existing user's privileges (stub).
func (c *UserController) UpgradeUser(ctx context.Context, req *proto.UpgradeUserReq) *proto.UpgradeUserResp {
	return &proto.UpgradeUserResp{
		Errno:  proto.ErrCode_ErrOk,
		Errmsg: proto.ErrCode_name[proto.ErrCode_ErrOk],
	}
}
