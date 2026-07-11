// Package converter also hosts Dubbo-specific converters, isolated in
// converter_dubbo.go to avoid clashing with the HTTP converter names.
package converter

import (
	"GS_PROJECT_MODULE/idl/dubbo/triple"
	"GS_PROJECT_MODULE/internal/application/order/dto"
)

// FromDubboCreateOrderReq converts a triple CreateOrderReq to an application DTO.
func FromDubboCreateOrderReq(req *triple.CreateOrderReq) *dto.CreateOrderReq {
	return &dto.CreateOrderReq{
		ID:     req.GetId(),
		UserID: req.GetUserId(),
		Amount: req.GetAmount(),
	}
}

// ToDubboOrder converts an order DTO to a triple Order model.
func ToDubboOrder(order *dto.OrderDTO) *triple.Order {
	return &triple.Order{
		Id:     order.ID,
		UserId: order.UserID,
		Amount: order.Amount,
		Status: order.Status,
	}
}
