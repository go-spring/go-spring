// Package converter converts between IDL request/response models and application DTOs.
package converter

import (
	"strconv"

	svc "GS_PROJECT_MODULE/idl/kitex-grpc/kitex_gen/svc"
	"GS_PROJECT_MODULE/internal/application/user/dto"
)

// FromKitexGrpcRegisterUserReq converts a Kitex/gRPC RegisterUserReq to an
// application DTO.
func FromKitexGrpcRegisterUserReq(req *svc.RegisterUserReq) *dto.CreateUserReq {
	return &dto.CreateUserReq{
		ID:    req.ID,
		Name:  req.Name,
		Email: req.Email,
	}
}

// ToKitexGrpcUser converts a user DTO plus request-carried fields into a
// Kitex/gRPC User model. email is not persisted on the DTO, so it is passed
// through from the request.
func ToKitexGrpcUser(user *dto.UserDTO, email string) *svc.User {
	return &svc.User{
		ID:    strconv.FormatInt(user.ID, 10),
		Name:  user.Name,
		Email: email,
	}
}
