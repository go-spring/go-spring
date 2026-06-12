package controller

import (
	"context"

	"GS_PROJECT_MODULE/idl/http/proto"
	"GS_PROJECT_MODULE/src/internal/user/internal/biz"
)

type UserController struct {
	Service *biz.UserService `autowire:""`
}

func (h *UserController) RegisterUser(ctx context.Context, req *proto.RegisterUserReq) *proto.RegisterUserResp {
	user, err := h.Service.RegisterUser(req.Id, req.Name, req.Email)
	if err != nil {
		return &proto.RegisterUserResp{
			Errno:  proto.ErrCode_PARAM_ERROR,
			Errmsg: proto.ErrCode_name[proto.ErrCode_PARAM_ERROR],
			Data:   nil,
		}
	}
	return &proto.RegisterUserResp{
		Errno:  proto.ErrCode_ErrOk,
		Errmsg: proto.ErrCode_name[proto.ErrCode_ErrOk],
		Data:   user,
	}
}

func (h *UserController) UpgradeUser(ctx context.Context, req *proto.UpgradeUserReq) *proto.UpgradeUserResp {
	user, err := h.Service.UpgradeUserLevel(req.Id)
	if err != nil {
		return &proto.UpgradeUserResp{
			Errno:  proto.ErrCode_PARAM_ERROR,
			Errmsg: proto.ErrCode_name[proto.ErrCode_PARAM_ERROR],
			Data:   nil,
		}
	}
	return &proto.UpgradeUserResp{
		Errno:  proto.ErrCode_ErrOk,
		Errmsg: proto.ErrCode_name[proto.ErrCode_ErrOk],
		Data:   user,
	}
}
