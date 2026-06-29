package order

import (
	"fmt"
	"time"
)

// Status represents the lifecycle state of an order.
type Status int32

const (
	StatusPending Status = iota
	StatusPaid
	StatusShipped
)

// Order is the aggregate root of the order domain.
// It enforces invariants over its internal Items and records domain events.
type Order struct {
	ID     string
	UserID int64
	Items  []OrderItem
	Total  Money
	Status Status
	events []any
}

// NewOrder creates a new order aggregate.
// The single product is wrapped into an Items slice to demonstrate
// the aggregate-root pattern while keeping the external API simple.
func NewOrder(userID int64, title string, amount float64) *Order {
	item := OrderItem{
		Title:    title,
		Price:    NewMoney(amount, ""),
		Quantity: 1,
	}
	return &Order{
		UserID: userID,
		Items:  []OrderItem{item},
		Total:  item.Subtotal(),
		Status: StatusPending,
	}
}

// Pay transitions the order from Pending to Paid and records a domain event.
func (o *Order) Pay() error {
	if o.Status != StatusPending {
		return fmt.Errorf("order %s cannot be paid: current status %v", o.ID, o.Status)
	}
	o.Status = StatusPaid
	o.events = append(o.events, OrderPaidEvent{OrderID: o.ID, PaidAt: time.Now()})
	return nil
}

// Ship transitions the order from Paid to Shipped.
func (o *Order) Ship() error {
	if o.Status != StatusPaid {
		return fmt.Errorf("order %s cannot be shipped: current status %v", o.ID, o.Status)
	}
	o.Status = StatusShipped
	return nil
}

// Events returns and clears accumulated domain events.
func (o *Order) Events() []any {
	evts := o.events
	o.events = nil
	return evts
}
