// Package converter also holds per-protocol converters. This file converts
// between the goframe gRPC pb types and the order application DTO; the HTTP
// converters live alongside it in converter.go.
package converter

import (
	"strconv"

	"GS_PROJECT_MODULE/idl/goframe-grpc/pb"
	"GS_PROJECT_MODULE/internal/application/order/dto"
)

// FromGoframeGrpcCreateOrderReq converts a pb CreateOrderReq to an application
// DTO. user_id arrives as a string on the wire and is parsed to int64.
func FromGoframeGrpcCreateOrderReq(req *pb.CreateOrderReq) *dto.CreateOrderReq {
	userID, _ := strconv.ParseInt(req.GetUserId(), 10, 64)
	return &dto.CreateOrderReq{
		ID:     req.GetId(),
		UserID: userID,
		Amount: req.GetAmount(),
	}
}

// ToGoframeGrpcOrder converts an order DTO to a pb Order.
func ToGoframeGrpcOrder(order *dto.OrderDTO) *pb.Order {
	return &pb.Order{
		Id:     order.ID,
		UserId: strconv.FormatInt(order.UserID, 10),
		Amount: order.Amount,
		Status: order.Status,
	}
}
