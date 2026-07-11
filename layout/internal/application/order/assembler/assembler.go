// Package assembler converts between domain entities and application DTOs for the order domain.
package assembler

import (
	"GS_PROJECT_MODULE/internal/application/order/dto"
	"GS_PROJECT_MODULE/internal/domain/order"
)

// ToEntity converts a create request to a domain Order entity.
func ToEntity(req *dto.CreateOrderReq) *order.Order {
	return order.NewOrder(req.UserID, req.Title, req.Amount)
}

// ToDTO converts a domain Order entity to a response DTO.
// It maps the aggregate-root internals back to the flat API model.
func ToDTO(order *order.Order) *dto.OrderDTO {
	title := ""
	amount := 0.0
	if len(order.Items) > 0 {
		title = order.Items[0].Title
		amount = order.Items[0].Price.Amount
	}
	return &dto.OrderDTO{
		ID:     order.ID,
		UserID: order.UserID,
		Title:  title,
		Amount: amount,
		Status: int32(order.Status),
	}
}

// ToDTOList converts a slice of domain Orders to response DTOs.
func ToDTOList(orders []*order.Order) []*dto.OrderDTO {
	result := make([]*dto.OrderDTO, len(orders))
	for i, o := range orders {
		result[i] = ToDTO(o)
	}
	return result
}
