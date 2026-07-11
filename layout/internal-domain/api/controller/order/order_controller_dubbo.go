// Package order also hosts the per-protocol controller adapters, distinguished
// by file suffix (order_controller_<proto>.go). This file is the Dubbo (triple)
// adapter; see order_controller.go for the HTTP baseline. All adapters delegate
// to the same order application service, so every protocol converges on one
// core.
package order

import (
	"context"

	"GS_PROJECT_MODULE/idl-domain/dubbo/triple"
	"GS_PROJECT_MODULE/internal-domain/api/controller/order/converter"
	"GS_PROJECT_MODULE/internal-domain/application/order"
)

// DubboOrderController adapts Dubbo (triple) order calls to the order
// application service. It consumes the triple-generated request/response types
// directly and converts to/from application DTOs at the boundary.
type DubboOrderController struct {
	OrderService *order.OrderService `autowire:""`
}

// CreateOrder converts a triple CreateOrderReq into a DTO, delegates to the
// application service, and maps the result back to a triple OrderResp.
func (c *DubboOrderController) CreateOrder(ctx context.Context, req *triple.CreateOrderReq) (*triple.OrderResp, error) {
	o, err := c.OrderService.CreateOrder(converter.FromDubboCreateOrderReq(req))
	if err != nil {
		return &triple.OrderResp{Code: 1, Message: err.Error()}, nil
	}
	return &triple.OrderResp{Code: 0, Message: "ok", Data: converter.ToDubboOrder(o)}, nil
}

// PayOrder delegates the payment request for an order.
func (c *DubboOrderController) PayOrder(ctx context.Context, req *triple.PayOrderReq) (*triple.OrderResp, error) {
	o, err := c.OrderService.PayOrder(req.GetId())
	if err != nil {
		return &triple.OrderResp{Code: 1, Message: err.Error()}, nil
	}
	return &triple.OrderResp{Code: 0, Message: "ok", Data: converter.ToDubboOrder(o)}, nil
}

// ShipOrder delegates the shipment request for an order.
func (c *DubboOrderController) ShipOrder(ctx context.Context, req *triple.ShipOrderReq) (*triple.OrderResp, error) {
	o, err := c.OrderService.ShipOrder(req.GetId())
	if err != nil {
		return &triple.OrderResp{Code: 1, Message: err.Error()}, nil
	}
	return &triple.OrderResp{Code: 0, Message: "ok", Data: converter.ToDubboOrder(o)}, nil
}
