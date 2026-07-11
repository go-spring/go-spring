package order

import "time"

// OrderPaidEvent is a domain event fired when an order transitions to Paid.
type OrderPaidEvent struct {
	OrderID string
	PaidAt  time.Time
}
