// Package converter converts between IDL request/response models and application DTOs.
package converter

import (
	"strconv"

	svc "GS_PROJECT_MODULE/idl/kitex-thrift/kitex_gen/svc"
	"GS_PROJECT_MODULE/internal/application/user/dto"
)

// FromKitexThriftRegisterUserReq converts a Kitex/Thrift RegisterUserReq to an
// application DTO.
func FromKitexThriftRegisterUserReq(req *svc.RegisterUserReq) *dto.CreateUserReq {
	return &dto.CreateUserReq{
		ID:    req.ID,
		Name:  req.Name,
		Email: req.Email,
	}
}

// ToKitexThriftUser converts a user DTO plus request-carried fields into a
// Kitex/Thrift User model. email is not persisted on the DTO, so it is passed
// through from the request.
func ToKitexThriftUser(user *dto.UserDTO, email string) *svc.User {
	return &svc.User{
		ID:    strconv.FormatInt(user.ID, 10),
		Name:  user.Name,
		Email: email,
	}
}
