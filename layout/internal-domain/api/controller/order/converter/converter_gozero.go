// Package converter converts between IDL request/response models and application DTOs.
package converter

import (
	"strconv"

	types "GS_PROJECT_MODULE/idl-domain/gozero/types"
	"GS_PROJECT_MODULE/internal-domain/application/order/dto"
)

// FromGozeroCreateOrderReq converts a go-zero CreateOrderReq (string UserId) to
// an application DTO (int64 UserID). Bad UserId values fall through as 0 to
// match the HTTP-proto converter's behavior.
func FromGozeroCreateOrderReq(req *types.CreateOrderReq) *dto.CreateOrderReq {
	userID, _ := strconv.ParseInt(req.UserId, 10, 64)
	return &dto.CreateOrderReq{
		ID:     req.Id,
		UserID: userID,
		Amount: req.Amount,
	}
}

// ToGozeroOrder converts an application OrderDTO to a go-zero Order (UserID
// int64 becomes string to fit JSON-safe numeric fields).
func ToGozeroOrder(o *dto.OrderDTO) *types.Order {
	return &types.Order{
		Id:     o.ID,
		UserId: strconv.FormatInt(o.UserID, 10),
		Amount: o.Amount,
		Status: o.Status,
	}
}
