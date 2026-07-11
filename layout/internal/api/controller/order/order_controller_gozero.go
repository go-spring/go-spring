// Package order also hosts the per-protocol controller adapters, distinguished
// by file suffix (order_controller_<proto>.go). This file is the go-zero adapter;
// see order_controller.go for the HTTP baseline. All adapters delegate to the
// same order application service, so every protocol converges on one core.
package order

import (
	"context"

	types "GS_PROJECT_MODULE/idl/gozero/types"
	"GS_PROJECT_MODULE/internal/api/controller/order/converter"
	"GS_PROJECT_MODULE/internal/application/order"
)

// GozeroOrderController adapts go-zero-native order calls to the order
// application service. Method signatures match the generated
// GS_PROJECT_NAMELogic interface so the composed server controller can satisfy
// it through embedding.
type GozeroOrderController struct {
	OrderService *order.OrderService `autowire:""`
}

// CreateOrder converts the go-zero request into an application DTO, delegates
// to OrderService, and maps the result back to the go-zero response envelope.
func (c *GozeroOrderController) CreateOrder(ctx context.Context, req *types.CreateOrderReq) (*types.OrderResp, error) {
	o, err := c.OrderService.CreateOrder(converter.FromGozeroCreateOrderReq(req))
	if err != nil {
		return &types.OrderResp{Code: 1, Message: err.Error()}, nil
	}
	return &types.OrderResp{Data: converter.ToGozeroOrder(o)}, nil
}

// PayOrder delegates the payment request for an order.
func (c *GozeroOrderController) PayOrder(ctx context.Context, req *types.PayOrderReq) (*types.OrderResp, error) {
	o, err := c.OrderService.PayOrder(req.Id)
	if err != nil {
		return &types.OrderResp{Code: 1, Message: err.Error()}, nil
	}
	return &types.OrderResp{Data: converter.ToGozeroOrder(o)}, nil
}

// ShipOrder delegates the shipment request for an order.
func (c *GozeroOrderController) ShipOrder(ctx context.Context, req *types.ShipOrderReq) (*types.OrderResp, error) {
	o, err := c.OrderService.ShipOrder(req.Id)
	if err != nil {
		return &types.OrderResp{Code: 1, Message: err.Error()}, nil
	}
	return &types.OrderResp{Data: converter.ToGozeroOrder(o)}, nil
}
