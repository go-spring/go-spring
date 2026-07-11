// Package converter converts between IDL request/response models and application DTOs.
package converter

import (
	"strconv"

	svc "GS_PROJECT_MODULE/idl-domain/kitex/kitex_gen/svc"
	"GS_PROJECT_MODULE/internal-domain/application/order/dto"
)

// FromKitexCreateOrderReq converts a Kitex CreateOrderReq to an application DTO.
// user_id is transported as a string over the wire and parsed into int64 here.
func FromKitexCreateOrderReq(req *svc.CreateOrderReq) *dto.CreateOrderReq {
	userID, _ := strconv.ParseInt(req.UserID, 10, 64)
	return &dto.CreateOrderReq{
		ID:     req.ID,
		UserID: userID,
		Amount: req.Amount,
	}
}

// ToKitexOrder converts an order DTO to a Kitex Order model.
func ToKitexOrder(order *dto.OrderDTO) *svc.Order {
	return &svc.Order{
		ID:     order.ID,
		UserID: strconv.FormatInt(order.UserID, 10),
		Amount: order.Amount,
		Status: order.Status,
	}
}
