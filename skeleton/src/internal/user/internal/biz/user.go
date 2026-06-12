package biz

import (
	"GS_PROJECT_MODULE/idl/http/proto"
	"GS_PROJECT_MODULE/src/internal/user/internal/dao"

	"github.com/go-spring/spring-core/gs"
)

func init() {
	gs.Object(&UserService{})
}

type UserService struct {
	Dao *dao.UserDao `autowire:""`
}

func (s *UserService) RegisterUser(id, username, email string) (*proto.User, error) {
	user := &proto.User{
		Id:    id,
		Name:  username,
		Email: email,
		Level: proto.UserLevel_Normal,
	}
	if err := s.Dao.Save(user); err != nil {
		return nil, err
	}
	return user, nil
}

func (s *UserService) UpgradeUserLevel(id string) (*proto.User, error) {
	user, err := s.Dao.FindByID(id)
	if err != nil {
		return nil, err
	}
	user.Level = proto.UserLevel_VIP
	if err = s.Dao.Save(user); err != nil {
		return nil, err
	}
	return user, nil
}
