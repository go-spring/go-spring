// Package order orchestrates order-related use cases, including cross-domain calls.
package order

import (
	"GS_PROJECT_MODULE/internal/application/order/assembler"
	"GS_PROJECT_MODULE/internal/application/order/dto"
	"GS_PROJECT_MODULE/internal/application/user"
	domainorder "GS_PROJECT_MODULE/internal/domain/order"
	orderrepo "GS_PROJECT_MODULE/internal/infra/repo/order"

	"time"

	"go-spring.org/spring/gs"
	"go-spring.org/stdlib/errutil"
)

func init() {
	gs.Provide(&OrderService{})
}

// OrderService orchestrates order creation, including cross-domain user validation.
type OrderService struct {
	OrderRepo   *orderrepo.Repo   `autowire:""`
	UserService *user.UserService `autowire:""`
}

// CreateOrder creates an order after verifying the user exists (cross-domain orchestration).
func (s *OrderService) CreateOrder(req *dto.CreateOrderReq) (*dto.OrderDTO, error) {
	// 1. Application-level pre-validation (distinct from domain invariants)
	if req.Title == "" {
		return nil, errutil.Explain(nil, "order title is required")
	}
	if req.Amount <= 0 {
		return nil, errutil.Explain(nil, "order amount must be positive")
	}

	// 2. Cross-domain call: verify user exists
	_, err := s.UserService.GetUser(req.UserID)
	if err != nil {
		return nil, errutil.Explain(err, "user verification failed")
	}

	// 3. ID allocation strategy (application concern)
	if req.ID == "" {
		req.ID = s.generateOrderID()
	}

	// 4. Create aggregate root
	order := assembler.ToEntity(req)

	// 5. Persist
	if err := s.OrderRepo.Save(order); err != nil {
		return nil, errutil.Stack(err, "order save failed")
	}

	// 6. Application-level side effects (logging, notifications)
	// TODO: publish OrderCreatedEvent to message bus

	return assembler.ToDTO(order), nil
}

// ListOrders queries orders by user with optional status filter and pagination.
// Demonstrates CQRS: read paths go through dedicated query methods.
func (s *OrderService) ListOrders(req *dto.ListOrdersReq) (*dto.ListOrdersResp, error) {
	q := orderrepo.Query{
		Limit:  req.Limit,
		Offset: req.Offset,
	}
	if req.Status >= 0 {
		s := domainorder.Status(req.Status)
		q.Status = &s
	}
	if req.UserID != 0 {
		q.UserID = &req.UserID
	}

	orders, err := s.OrderRepo.Find(q)
	if err != nil {
		return nil, errutil.Stack(err, "query order list failed")
	}

	return &dto.ListOrdersResp{
		Items: assembler.ToDTOList(orders),
		Total: int64(len(orders)),
	}, nil
}

// BatchPayOrders processes payment for multiple orders.
// Demonstrates application-level collection handling and partial failure.
func (s *OrderService) BatchPayOrders(req *dto.BatchPayReq) *dto.BatchPayResp {
	results := make([]*dto.BatchPayResult, 0, len(req.OrderIDs))

	for _, id := range req.OrderIDs {
		order, err := s.OrderRepo.FindByID(id)
		if err != nil {
			results = append(results, &dto.BatchPayResult{
				OrderID: id,
				Success: false,
				ErrMsg:  err.Error(),
			})
			continue
		}

		if err := order.Pay(); err != nil {
			results = append(results, &dto.BatchPayResult{
				OrderID: id,
				Success: false,
				ErrMsg:  err.Error(),
			})
			continue
		}

		if err := s.OrderRepo.Update(order); err != nil {
			results = append(results, &dto.BatchPayResult{
				OrderID: id,
				Success: false,
				ErrMsg:  err.Error(),
			})
			continue
		}

		// Publish domain events
		for _, evt := range order.Events() {
			switch e := evt.(type) {
			case domainorder.OrderPaidEvent:
				_ = e // TODO: publish to event bus
			}
		}

		results = append(results, &dto.BatchPayResult{
			OrderID: id,
			Success: true,
		})
	}

	return &dto.BatchPayResp{Results: results}
}

// PayOrder delegates the payment transition to the domain entity and publishes events.
func (s *OrderService) PayOrder(orderID string) (*dto.OrderDTO, error) {
	order, err := s.OrderRepo.FindByID(orderID)
	if err != nil {
		return nil, errutil.Stack(err, "PayOrder(%s)", orderID)
	}
	if err := order.Pay(); err != nil {
		return nil, errutil.Stack(err, "PayOrder(%s)", orderID)
	}
	if err := s.OrderRepo.Update(order); err != nil {
		return nil, errutil.Stack(err, "PayOrder(%s)", orderID)
	}

	// Publish domain events (integrate with message bus or event store)
	for _, evt := range order.Events() {
		switch e := evt.(type) {
		case domainorder.OrderPaidEvent:
			_ = e // TODO: publish to event bus
		}
	}

	return assembler.ToDTO(order), nil
}

// ShipOrder delegates the shipment transition to the domain entity.
func (s *OrderService) ShipOrder(orderID string) (*dto.OrderDTO, error) {
	order, err := s.OrderRepo.FindByID(orderID)
	if err != nil {
		return nil, errutil.Stack(err, "ShipOrder(%s)", orderID)
	}
	if err := order.Ship(); err != nil {
		return nil, errutil.Stack(err, "ShipOrder(%s)", orderID)
	}
	if err := s.OrderRepo.Update(order); err != nil {
		return nil, errutil.Stack(err, "ShipOrder(%s)", orderID)
	}
	return assembler.ToDTO(order), nil
}

// generateOrderID is an application-level ID allocation strategy.
func (s *OrderService) generateOrderID() string {
	return "ORD-" + time.Now().Format("20060102-150405-999")
}
