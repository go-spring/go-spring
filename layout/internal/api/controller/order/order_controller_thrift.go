// Package order also hosts the per-protocol controller adapters, distinguished
// by file suffix (order_controller_<proto>.go). This file is the Thrift adapter;
// see order_controller.go for the HTTP baseline. All adapters delegate to the
// same order application service, so every protocol converges on one core.
package order

import (
	"context"

	thrift "GS_PROJECT_MODULE/idl/thrift/gen"
	"GS_PROJECT_MODULE/internal/api/controller/order/converter"
	"GS_PROJECT_MODULE/internal/application/order"
)

// ThriftOrderController adapts Thrift-generated order calls to the order
// application service. The method signatures are thrift-native so the composed
// controller in thriftsvr can directly satisfy thrift.GS_PROJECT_NAMEService.
type ThriftOrderController struct {
	OrderService *order.OrderService `autowire:""`
}

// CreateOrder converts the Thrift request to a DTO, delegates to OrderService,
// and maps the result back to a Thrift OrderResp.
func (c *ThriftOrderController) CreateOrder(ctx context.Context, req *thrift.CreateOrderReq) (*thrift.OrderResp, error) {
	resp := &thrift.OrderResp{}
	o, err := c.OrderService.CreateOrder(converter.FromThriftCreateOrderReq(req))
	if err != nil {
		resp.Code = 1
		resp.Message = err.Error()
		return resp, nil
	}
	resp.Data = converter.ToThriftOrder(o)
	return resp, nil
}

// PayOrder handles the payment request for an order.
func (c *ThriftOrderController) PayOrder(ctx context.Context, req *thrift.PayOrderReq) (*thrift.OrderResp, error) {
	resp := &thrift.OrderResp{}
	o, err := c.OrderService.PayOrder(req.ID)
	if err != nil {
		resp.Code = 1
		resp.Message = err.Error()
		return resp, nil
	}
	resp.Data = converter.ToThriftOrder(o)
	return resp, nil
}

// ShipOrder handles the shipment request for an order.
func (c *ThriftOrderController) ShipOrder(ctx context.Context, req *thrift.ShipOrderReq) (*thrift.OrderResp, error) {
	resp := &thrift.OrderResp{}
	o, err := c.OrderService.ShipOrder(req.ID)
	if err != nil {
		resp.Code = 1
		resp.Message = err.Error()
		return resp, nil
	}
	resp.Data = converter.ToThriftOrder(o)
	return resp, nil
}
