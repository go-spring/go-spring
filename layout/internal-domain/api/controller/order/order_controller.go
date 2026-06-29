// Package order is the HTTP controller layer for the order domain.
// It validates incoming requests, delegates to application services, and formats responses.
package order

import (
	"context"

	"GS_PROJECT_MODULE/idl-domain/http/proto"
	"GS_PROJECT_MODULE/internal-domain/api/controller/order/converter"
	"GS_PROJECT_MODULE/internal-domain/application/order"
)

// OrderController implements order-related API handlers.
type OrderController struct {
	OrderService *order.OrderService `autowire:""`
}

// CreateOrder converts the HTTP request to a DTO, delegates to OrderService,
// and maps the result back to a proto response.
func (c *OrderController) CreateOrder(ctx context.Context, req *proto.CreateOrderReq) *proto.CreateOrderResp {
	resp := &proto.CreateOrderResp{
		Errno:  proto.ErrCode_ErrOk,
		Errmsg: proto.ErrCode_name[proto.ErrCode_ErrOk],
	}

	order, err := c.OrderService.CreateOrder(converter.FromCreateOrderReq(req))
	if err != nil {
		resp.Errno = proto.ErrCode_PARAM_ERROR
		resp.Errmsg = err.Error()
		return resp
	}

	resp.Data = converter.ToProtoOrder(order)
	return resp
}

// PayOrder handles the payment request for an order.
func (c *OrderController) PayOrder(ctx context.Context, req *proto.PayOrderReq) *proto.PayOrderResp {
	resp := &proto.PayOrderResp{
		Errno:  proto.ErrCode_ErrOk,
		Errmsg: proto.ErrCode_name[proto.ErrCode_ErrOk],
	}
	order, err := c.OrderService.PayOrder(req.Id)
	if err != nil {
		resp.Errno = proto.ErrCode_PARAM_ERROR
		resp.Errmsg = err.Error()
		return resp
	}
	resp.Data = converter.ToProtoOrder(order)
	return resp
}

// ShipOrder handles the shipment request for an order.
func (c *OrderController) ShipOrder(ctx context.Context, req *proto.ShipOrderReq) *proto.ShipOrderResp {
	resp := &proto.ShipOrderResp{
		Errno:  proto.ErrCode_ErrOk,
		Errmsg: proto.ErrCode_name[proto.ErrCode_ErrOk],
	}
	order, err := c.OrderService.ShipOrder(req.Id)
	if err != nil {
		resp.Errno = proto.ErrCode_PARAM_ERROR
		resp.Errmsg = err.Error()
		return resp
	}
	resp.Data = converter.ToProtoOrder(order)
	return resp
}
