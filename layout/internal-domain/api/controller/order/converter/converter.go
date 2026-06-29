// Package converter converts between IDL request/response models and application DTOs.
package converter

import (
	"strconv"

	"GS_PROJECT_MODULE/idl-domain/http/proto"
	"GS_PROJECT_MODULE/internal-domain/application/order/dto"
)

// FromCreateOrderReq converts a proto CreateOrderReq to an application DTO.
func FromCreateOrderReq(req *proto.CreateOrderReq) *dto.CreateOrderReq {
	userID, _ := strconv.ParseInt(req.UserId, 10, 64)
	return &dto.CreateOrderReq{
		ID:     req.Id,
		UserID: userID,
		Amount: req.Amount,
	}
}

// ToProtoOrder converts an order DTO to a proto Order model.
func ToProtoOrder(order *dto.OrderDTO) *proto.Order {
	return &proto.Order{
		Id:     order.ID,
		UserId: strconv.FormatInt(order.UserID, 10),
		Amount: order.Amount,
		Status: proto.OrderStatus(order.Status),
	}
}
