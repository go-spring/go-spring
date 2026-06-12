package dao

import (
	"GS_PROJECT_MODULE/idl/http/proto"

	"github.com/go-spring/spring-core/gs"
	"gorm.io/gorm"
)

func init() {
	gs.Object(&OrderDao{})
}

type OrderDao struct {
	DB *gorm.DB `autowire:""`
}

func (r *OrderDao) Save(order *proto.Order) error {
	db := r.DB.Exec(
		"REPLACE INTO orders (id, user_id, amount, status) VALUES (?, ?, ?, ?)",
		order.Id, order.UserId, order.Amount, order.Status,
	)
	return db.Error
}

func (r *OrderDao) FindByID(id string) (*proto.Order, error) {
	db := r.DB.Exec("SELECT id, user_id, amount, status FROM orders WHERE id = ?", id)
	if err := db.Error; err != nil {
		return nil, err
	}
	var o proto.Order
	if err := db.Scan(&o).Error; err != nil {
		return nil, err
	}
	return &o, nil
}
