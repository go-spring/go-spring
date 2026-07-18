// Package order also hosts the per-protocol controller adapters, distinguished
// by file suffix (order_controller_<proto>.go). This file is the goframe gRPC
// adapter; it consumes the generated pb request/response types directly and
// delegates to the shared order application service.
package order

import (
	"context"

	"GS_PROJECT_MODULE/idl/goframe-grpc/pb"
	"GS_PROJECT_MODULE/internal/api/controller/order/converter"
	"GS_PROJECT_MODULE/internal/application/order"
)

// GoframeGrpcOrderController adapts goframe gRPC order rpcs to the order
// application service. It converts pb messages into DTOs on the way in and
// back into pb messages on the way out; the application service never sees pb
// types.
type GoframeGrpcOrderController struct {
	OrderService *order.OrderService `autowire:""`
}

// CreateOrder converts the pb request to a DTO, delegates to OrderService, and
// wraps the result in an OrderResp envelope.
func (c *GoframeGrpcOrderController) CreateOrder(ctx context.Context, req *pb.CreateOrderReq) (*pb.OrderResp, error) {
	o, err := c.OrderService.CreateOrder(converter.FromGoframeGrpcCreateOrderReq(req))
	if err != nil {
		return &pb.OrderResp{Code: 1, Message: err.Error()}, nil
	}
	return &pb.OrderResp{Code: 0, Message: "ok", Data: converter.ToGoframeGrpcOrder(o)}, nil
}

// PayOrder handles the payment rpc for an order.
func (c *GoframeGrpcOrderController) PayOrder(ctx context.Context, req *pb.PayOrderReq) (*pb.OrderResp, error) {
	o, err := c.OrderService.PayOrder(req.GetId())
	if err != nil {
		return &pb.OrderResp{Code: 1, Message: err.Error()}, nil
	}
	return &pb.OrderResp{Code: 0, Message: "ok", Data: converter.ToGoframeGrpcOrder(o)}, nil
}

// ShipOrder handles the shipment rpc for an order.
func (c *GoframeGrpcOrderController) ShipOrder(ctx context.Context, req *pb.ShipOrderReq) (*pb.OrderResp, error) {
	o, err := c.OrderService.ShipOrder(req.GetId())
	if err != nil {
		return &pb.OrderResp{Code: 1, Message: err.Error()}, nil
	}
	return &pb.OrderResp{Code: 0, Message: "ok", Data: converter.ToGoframeGrpcOrder(o)}, nil
}
