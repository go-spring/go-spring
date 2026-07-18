// Package converter also holds per-protocol converters. This file converts
// between the goframe HTTP pb types and the order application DTO; the base
// HTTP converters live alongside it in converter.go.
package converter

import (
	"strconv"

	"GS_PROJECT_MODULE/idl/goframe-http/pb"
	"GS_PROJECT_MODULE/internal/application/order/dto"
)

// FromGoframeHttpCreateOrderReq converts a pb CreateOrderReq to an application
// DTO. user_id arrives as a string on the wire and is parsed to int64.
func FromGoframeHttpCreateOrderReq(req *pb.CreateOrderReq) *dto.CreateOrderReq {
	userID, _ := strconv.ParseInt(req.GetUserId(), 10, 64)
	return &dto.CreateOrderReq{
		ID:     req.GetId(),
		UserID: userID,
		Amount: req.GetAmount(),
	}
}

// ToGoframeHttpOrder converts an order DTO to a pb Order.
func ToGoframeHttpOrder(order *dto.OrderDTO) *pb.Order {
	return &pb.Order{
		Id:     order.ID,
		UserId: strconv.FormatInt(order.UserID, 10),
		Amount: order.Amount,
		Status: order.Status,
	}
}
