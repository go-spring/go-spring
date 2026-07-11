// Package converter converts between IDL request/response models and application DTOs.
package converter

import (
	"strconv"

	types "GS_PROJECT_MODULE/idl-domain/gozero/types"
	"GS_PROJECT_MODULE/internal-domain/application/user/dto"
)

// FromGozeroRegisterUserReq converts a go-zero RegisterUserReq to an
// application CreateUserReq DTO.
func FromGozeroRegisterUserReq(req *types.RegisterUserReq) *dto.CreateUserReq {
	return &dto.CreateUserReq{
		ID:    req.Id,
		Name:  req.Name,
		Email: req.Email,
	}
}

// ToGozeroUser converts an application UserDTO to a go-zero User. Email is
// carried on the request side because UserDTO does not persist it.
func ToGozeroUser(u *dto.UserDTO, email string) *types.User {
	return &types.User{
		Id:    strconv.FormatInt(u.ID, 10),
		Name:  u.Name,
		Email: email,
	}
}
