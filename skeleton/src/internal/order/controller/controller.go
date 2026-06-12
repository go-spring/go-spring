package controller

import (
	"context"

	"GS_PROJECT_MODULE/idl/http/proto"
	"GS_PROJECT_MODULE/src/internal/order/internal/biz"
)

type OrderController struct {
	Service *biz.OrderService `autowire:""`
}

func (h *OrderController) CreateOrder(ctx context.Context, req *proto.CreateOrderReq) *proto.CreateOrderResp {
	order, err := h.Service.CreateOrder(req.Id, req.Id, req.Amount)
	if err != nil {
		return &proto.CreateOrderResp{
			Errno:  proto.ErrCode_PARAM_ERROR,
			Errmsg: proto.ErrCode_name[proto.ErrCode_PARAM_ERROR],
			Data:   nil,
		}
	}
	return &proto.CreateOrderResp{
		Errno:  proto.ErrCode_ErrOk,
		Errmsg: proto.ErrCode_name[proto.ErrCode_ErrOk],
		Data:   order,
	}
}

func (h *OrderController) PayOrder(ctx context.Context, req *proto.PayOrderReq) *proto.PayOrderResp {
	order, err := h.Service.PayOrder(req.Id)
	if err != nil {
		return &proto.PayOrderResp{
			Errno:  proto.ErrCode_PARAM_ERROR,
			Errmsg: proto.ErrCode_name[proto.ErrCode_PARAM_ERROR],
			Data:   nil,
		}
	}
	return &proto.PayOrderResp{
		Errno:  proto.ErrCode_ErrOk,
		Errmsg: proto.ErrCode_name[proto.ErrCode_ErrOk],
		Data:   order,
	}
}

func (h *OrderController) ShipOrder(ctx context.Context, req *proto.ShipOrderReq) *proto.ShipOrderResp {
	order, err := h.Service.ShipOrder(req.Id)
	if err != nil {
		return &proto.ShipOrderResp{
			Errno:  proto.ErrCode_PARAM_ERROR,
			Errmsg: proto.ErrCode_name[proto.ErrCode_PARAM_ERROR],
			Data:   nil,
		}
	}
	return &proto.ShipOrderResp{
		Errno:  proto.ErrCode_ErrOk,
		Errmsg: proto.ErrCode_name[proto.ErrCode_ErrOk],
		Data:   order,
	}
}
