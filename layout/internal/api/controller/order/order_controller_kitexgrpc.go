// Package order also hosts the per-protocol controller adapters, distinguished
// by file suffix (order_controller_<proto>.go). This file is the Kitex/gRPC
// adapter; see order_controller.go for the HTTP baseline. All adapters delegate
// to the same order application service, so every protocol converges on one core.
package order

import (
	"context"

	svc "GS_PROJECT_MODULE/idl/kitex-grpc/kitex_gen/svc"
	"GS_PROJECT_MODULE/internal/api/controller/order/converter"
	"GS_PROJECT_MODULE/internal/application/order"
)

// KitexGrpcOrderController adapts Kitex-generated (gRPC/protobuf) order calls
// to the order application service. The method signatures are kitex-native so
// the composed controller in kitexgrpcsvr can directly satisfy
// svc.GS_PROJECT_NAMEService.
type KitexGrpcOrderController struct {
	OrderService *order.OrderService `autowire:""`
}

// CreateOrder converts the Kitex request to a DTO, delegates to OrderService,
// and maps the result back to a Kitex OrderResp.
func (c *KitexGrpcOrderController) CreateOrder(ctx context.Context, req *svc.CreateOrderReq) (*svc.OrderResp, error) {
	resp := &svc.OrderResp{}
	o, err := c.OrderService.CreateOrder(converter.FromKitexGrpcCreateOrderReq(req))
	if err != nil {
		resp.Code = 1
		resp.Message = err.Error()
		return resp, nil
	}
	resp.Data = converter.ToKitexGrpcOrder(o)
	return resp, nil
}

// PayOrder handles the payment request for an order.
func (c *KitexGrpcOrderController) PayOrder(ctx context.Context, req *svc.PayOrderReq) (*svc.OrderResp, error) {
	resp := &svc.OrderResp{}
	o, err := c.OrderService.PayOrder(req.ID)
	if err != nil {
		resp.Code = 1
		resp.Message = err.Error()
		return resp, nil
	}
	resp.Data = converter.ToKitexGrpcOrder(o)
	return resp, nil
}

// ShipOrder handles the shipment request for an order.
func (c *KitexGrpcOrderController) ShipOrder(ctx context.Context, req *svc.ShipOrderReq) (*svc.OrderResp, error) {
	resp := &svc.OrderResp{}
	o, err := c.OrderService.ShipOrder(req.ID)
	if err != nil {
		resp.Code = 1
		resp.Message = err.Error()
		return resp, nil
	}
	resp.Data = converter.ToKitexGrpcOrder(o)
	return resp, nil
}
