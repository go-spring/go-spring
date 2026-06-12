package biz

import (
	"GS_PROJECT_MODULE/idl/http/proto"
	"GS_PROJECT_MODULE/src/internal/order/internal/dao"

	"go-spring.org/spring/gs"
)

func init() {
	gs.Object(&OrderService{})
}

type OrderService struct {
	Dao *dao.OrderDao `autowire:""`
}

func (s *OrderService) CreateOrder(id, userID string, amount float64) (*proto.Order, error) {
	order := &proto.Order{
		Id:     id,
		UserId: userID,
		Amount: amount,
	}
	if err := s.Dao.Save(order); err != nil {
		return nil, err
	}
	return order, nil
}

func (s *OrderService) PayOrder(id string) (*proto.Order, error) {
	order, err := s.Dao.FindByID(id)
	if err != nil {
		return nil, err
	}
	order.Status = proto.OrderStatus_Paid
	if err = s.Dao.Save(order); err != nil {
		return nil, err
	}
	return order, nil
}

func (s *OrderService) ShipOrder(id string) (*proto.Order, error) {
	order, err := s.Dao.FindByID(id)
	if err != nil {
		return nil, err
	}
	order.Status = proto.OrderStatus_Shipped
	if err = s.Dao.Save(order); err != nil {
		return nil, err
	}
	return order, nil
}
