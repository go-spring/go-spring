// Package converter converts between IDL request/response models and application DTOs.
package converter

import (
	"strconv"

	thrift "GS_PROJECT_MODULE/idl/thrift/gen"
	"GS_PROJECT_MODULE/internal/application/order/dto"
)

// FromThriftCreateOrderReq converts a Thrift CreateOrderReq to an application DTO.
// user_id is transported as a string over the wire and parsed into int64 here.
func FromThriftCreateOrderReq(req *thrift.CreateOrderReq) *dto.CreateOrderReq {
	userID, _ := strconv.ParseInt(req.UserID, 10, 64)
	return &dto.CreateOrderReq{
		ID:     req.ID,
		UserID: userID,
		Amount: req.Amount,
	}
}

// ToThriftOrder converts an order DTO to a Thrift Order model.
func ToThriftOrder(order *dto.OrderDTO) *thrift.Order {
	return &thrift.Order{
		ID:     order.ID,
		UserID: strconv.FormatInt(order.UserID, 10),
		Amount: order.Amount,
		Status: order.Status,
	}
}
