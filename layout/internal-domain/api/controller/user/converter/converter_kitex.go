// Package converter converts between IDL request/response models and application DTOs.
package converter

import (
	"strconv"

	svc "GS_PROJECT_MODULE/idl-domain/kitex/kitex_gen/svc"
	"GS_PROJECT_MODULE/internal-domain/application/user/dto"
)

// FromKitexRegisterUserReq converts a Kitex RegisterUserReq to an application DTO.
func FromKitexRegisterUserReq(req *svc.RegisterUserReq) *dto.CreateUserReq {
	return &dto.CreateUserReq{
		ID:    req.ID,
		Name:  req.Name,
		Email: req.Email,
	}
}

// ToKitexUser converts a user DTO plus request-carried fields into a Kitex User model.
// email is not persisted on the DTO, so it is passed through from the request.
func ToKitexUser(user *dto.UserDTO, email string) *svc.User {
	return &svc.User{
		ID:    strconv.FormatInt(user.ID, 10),
		Name:  user.Name,
		Email: email,
	}
}
