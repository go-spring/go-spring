// Package converter also holds per-protocol converters. This file converts
// between the kratos-HTTP pb types and the user application DTO.
package converter

import (
	"GS_PROJECT_MODULE/idl/kratos-http/pb"
	"GS_PROJECT_MODULE/internal/application/user/dto"
)

// FromKratosHttpRegisterUserReq converts a kratoshttp pb RegisterUserReq to an
// application DTO.
func FromKratosHttpRegisterUserReq(req *pb.RegisterUserReq) *dto.CreateUserReq {
	return &dto.CreateUserReq{
		ID:    req.GetId(),
		Name:  req.GetName(),
		Email: req.GetEmail(),
	}
}

// ToKratosHttpUser converts a user DTO and request-specific email to a
// kratoshttp pb User.
func ToKratosHttpUser(user *dto.UserDTO, email string) *pb.User {
	return &pb.User{
		Id:    user.ID,
		Name:  user.Name,
		Email: email,
	}
}
