// Package converter converts between IDL request/response models and application DTOs.
package converter

import (
	"strconv"

	svc "GS_PROJECT_MODULE/idl/kitex-grpc/kitex_gen/svc"
	"GS_PROJECT_MODULE/internal/application/order/dto"
)

// FromKitexGrpcCreateOrderReq converts a Kitex/gRPC CreateOrderReq to an
// application DTO. user_id is transported as a string over the wire and parsed
// into int64 here.
func FromKitexGrpcCreateOrderReq(req *svc.CreateOrderReq) *dto.CreateOrderReq {
	userID, _ := strconv.ParseInt(req.UserID, 10, 64)
	return &dto.CreateOrderReq{
		ID:     req.ID,
		UserID: userID,
		Amount: req.Amount,
	}
}

// ToKitexGrpcOrder converts an order DTO to a Kitex/gRPC Order model.
func ToKitexGrpcOrder(order *dto.OrderDTO) *svc.Order {
	return &svc.Order{
		ID:     order.ID,
		UserID: strconv.FormatInt(order.UserID, 10),
		Amount: order.Amount,
		Status: order.Status,
	}
}
