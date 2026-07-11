package order

// Money is a value object representing a monetary amount with currency.
type Money struct {
	Amount   float64
	Currency string
}

// NewMoney creates a Money value object with the given amount and currency.
// Empty currency defaults to "CNY".
func NewMoney(amount float64, currency string) Money {
	if currency == "" {
		currency = "CNY"
	}
	return Money{Amount: amount, Currency: currency}
}

// OrderItem is a value object representing a line item within an order.
type OrderItem struct {
	ProductID string
	Title     string
	Price     Money
	Quantity  int
}

// Subtotal returns the line total (Price * Quantity).
func (i OrderItem) Subtotal() Money {
	return NewMoney(i.Price.Amount*float64(i.Quantity), i.Price.Currency)
}
