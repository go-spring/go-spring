// Package dto defines order request/response models used by the application layer.
package dto

// CreateOrderReq is the request DTO for creating an order.
type CreateOrderReq struct {
	ID     string // business order ID from the external request; auto-generated if empty
	UserID int64
	Title  string
	Amount float64
}

// OrderDTO is the response DTO for a single order.
type OrderDTO struct {
	ID     string
	UserID int64
	Title  string
	Amount float64
	Status int32
}

// ListOrdersReq is the request DTO for querying orders.
type ListOrdersReq struct {
	UserID int64
	Status int32 // -1 means no filter
	Limit  int
	Offset int
}

// ListOrdersResp is the response DTO for order queries.
type ListOrdersResp struct {
	Total int64
	Items []*OrderDTO
}

// BatchPayReq is the request DTO for batch payment.
type BatchPayReq struct {
	OrderIDs []string
}

// BatchPayResult is the result item for a single order in a batch operation.
type BatchPayResult struct {
	OrderID string
	Success bool
	ErrMsg  string
}

// BatchPayResp is the response DTO for batch payment.
type BatchPayResp struct {
	Results []*BatchPayResult
}
