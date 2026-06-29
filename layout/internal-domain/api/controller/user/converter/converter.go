// Package converter converts between IDL request/response models and application DTOs.
package converter

import (
	"strconv"

	"GS_PROJECT_MODULE/idl-domain/http/proto"
	"GS_PROJECT_MODULE/internal-domain/application/user/dto"
)

// FromRegisterUserReq converts a proto RegisterUserReq to an application DTO.
func FromRegisterUserReq(req *proto.RegisterUserReq) *dto.CreateUserReq {
	return &dto.CreateUserReq{
		ID:    req.Id,
		Name:  req.Name,
		Email: req.Email,
	}
}

// ToProtoUser converts a user DTO and request-specific fields to a proto User model.
func ToProtoUser(user *dto.UserDTO, email string) *proto.User {
	return &proto.User{
		Id:    strconv.FormatInt(user.ID, 10),
		Name:  user.Name,
		Email: email,
	}
}
