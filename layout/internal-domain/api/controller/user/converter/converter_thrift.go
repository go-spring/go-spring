// Package converter converts between IDL request/response models and application DTOs.
package converter

import (
	"strconv"

	thrift "GS_PROJECT_MODULE/idl-domain/thrift/gen"
	"GS_PROJECT_MODULE/internal-domain/application/user/dto"
)

// FromThriftRegisterUserReq converts a Thrift RegisterUserReq to an application DTO.
func FromThriftRegisterUserReq(req *thrift.RegisterUserReq) *dto.CreateUserReq {
	return &dto.CreateUserReq{
		ID:    req.ID,
		Name:  req.Name,
		Email: req.Email,
	}
}

// ToThriftUser converts a user DTO and request-specific fields to a Thrift User model.
func ToThriftUser(user *dto.UserDTO, email string) *thrift.User {
	return &thrift.User{
		ID:    strconv.FormatInt(user.ID, 10),
		Name:  user.Name,
		Email: email,
	}
}
