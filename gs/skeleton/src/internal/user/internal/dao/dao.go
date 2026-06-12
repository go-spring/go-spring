package dao

import (
	"GS_PROJECT_MODULE/idl/http/proto"

	"go-spring.org/spring/gs"
	"gorm.io/gorm"
)

func init() {
	gs.Object(&UserDao{})
}

type UserDao struct {
	DB *gorm.DB `autowire:""`
}

func (r *UserDao) Save(user *proto.User) error {
	db := r.DB.Exec(
		"INSERT OR REPLACE INTO users (id, username, email, level) VALUES (?, ?, ?, ?)",
		user.Id, user.Name, user.Email, user.Level,
	)
	return db.Error
}

func (r *UserDao) FindByID(id string) (*proto.User, error) {
	db := r.DB.Exec("SELECT id, username, email, level FROM users WHERE id = ?", id)
	if err := db.Error; err != nil {
		return nil, err
	}
	var u proto.User
	if err := db.Scan(&u).Error; err != nil {
		return nil, err
	}
	return &u, nil
}
