// Package converter also holds per-protocol converters. This file converts
// between the kratos-gRPC pb types and the order application DTO.
package converter

import (
	"strconv"

	"GS_PROJECT_MODULE/idl/kratos-grpc/pb"
	"GS_PROJECT_MODULE/internal/application/order/dto"
)

// FromKratosGrpcCreateOrderReq converts a kratosgrpc pb CreateOrderReq to an
// application DTO. user_id arrives as a string on the wire and is parsed to
// int64.
func FromKratosGrpcCreateOrderReq(req *pb.CreateOrderReq) *dto.CreateOrderReq {
	userID, _ := strconv.ParseInt(req.GetUserId(), 10, 64)
	return &dto.CreateOrderReq{
		ID:     req.GetId(),
		UserID: userID,
		Amount: req.GetAmount(),
	}
}

// ToKratosGrpcOrder converts an order DTO to a kratosgrpc pb Order.
func ToKratosGrpcOrder(order *dto.OrderDTO) *pb.Order {
	return &pb.Order{
		Id:     order.ID,
		UserId: strconv.FormatInt(order.UserID, 10),
		Amount: order.Amount,
		Status: order.Status,
	}
}
