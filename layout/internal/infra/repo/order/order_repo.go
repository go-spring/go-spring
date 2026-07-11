// Package order provides a GORM-backed implementation of the order repository.
// Delete this package (and its starter import) if you don't need a database.
package order

import (
	"errors"

	"GS_PROJECT_MODULE/internal/domain/order"

	"go-spring.org/spring/gs"
	"go-spring.org/stdlib/errutil"
	"gorm.io/gorm"
)

func init() {
	gs.Provide(&Repo{}).Init((*Repo).migrate)
}

// orderPO is the persistence object mapped to the "orders" table.
// The aggregate's single line item is flattened into columns to keep
// the example schema simple; a real system would use a child table.
type orderPO struct {
	ID       string `gorm:"primaryKey;size:64"`
	UserID   int64  `gorm:"index"`
	Title    string `gorm:"size:255"`
	Amount   float64
	Currency string `gorm:"size:8"`
	Status   int32
}

// TableName sets the mapped table name for orderPO.
func (orderPO) TableName() string { return "orders" }

func toPO(o *order.Order) *orderPO {
	title, amount, currency := "", 0.0, ""
	if len(o.Items) > 0 {
		item := o.Items[0]
		title = item.Title
		amount = item.Price.Amount
		currency = item.Price.Currency
	}
	return &orderPO{
		ID:       o.ID,
		UserID:   o.UserID,
		Title:    title,
		Amount:   amount,
		Currency: currency,
		Status:   int32(o.Status),
	}
}

func (po *orderPO) toDomain() *order.Order {
	item := order.OrderItem{
		Title:    po.Title,
		Price:    order.NewMoney(po.Amount, po.Currency),
		Quantity: 1,
	}
	return &order.Order{
		ID:     po.ID,
		UserID: po.UserID,
		Items:  []order.OrderItem{item},
		Total:  item.Subtotal(),
		Status: order.Status(po.Status),
	}
}

// Query packages filtering criteria for order queries.
// Nil pointer fields mean "no restriction".
type Query struct {
	UserID *int64
	Status *order.Status
	Limit  int
	Offset int
}

// Repo is a GORM-backed implementation of the order repository.
type Repo struct {
	DB *gorm.DB `autowire:""`
}

// migrate ensures the underlying table exists. It runs after dependency injection.
func (r *Repo) migrate() error {
	return r.DB.AutoMigrate(&orderPO{})
}

// Save stores a new order.
func (r *Repo) Save(o *order.Order) error {
	if err := r.DB.Create(toPO(o)).Error; err != nil {
		return errutil.Stack(err, "save order")
	}
	return nil
}

// FindByID retrieves an order by its business ID.
func (r *Repo) FindByID(id string) (*order.Order, error) {
	var po orderPO
	if err := r.DB.First(&po, "id = ?", id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errutil.Explain(nil, "order %s not found", id)
		}
		return nil, errutil.Stack(err, "find order %s", id)
	}
	return po.toDomain(), nil
}

// Update persists changes to an existing order.
func (r *Repo) Update(o *order.Order) error {
	res := r.DB.Model(&orderPO{}).Where("id = ?", o.ID).Updates(toPO(o))
	if res.Error != nil {
		return errutil.Stack(res.Error, "update order %s", o.ID)
	}
	if res.RowsAffected == 0 {
		return errutil.Explain(nil, "order %s not found", o.ID)
	}
	return nil
}

// Find returns orders matching the given query, with pagination.
func (r *Repo) Find(q Query) ([]*order.Order, error) {
	db := r.DB.Model(&orderPO{})
	if q.UserID != nil {
		db = db.Where("user_id = ?", *q.UserID)
	}
	if q.Status != nil {
		db = db.Where("status = ?", int32(*q.Status))
	}
	if q.Offset > 0 {
		db = db.Offset(q.Offset)
	}
	if q.Limit > 0 {
		db = db.Limit(q.Limit)
	}

	var pos []orderPO
	if err := db.Find(&pos).Error; err != nil {
		return nil, errutil.Stack(err, "find orders")
	}

	result := make([]*order.Order, len(pos))
	for i := range pos {
		result[i] = pos[i].toDomain()
	}
	return result, nil
}
