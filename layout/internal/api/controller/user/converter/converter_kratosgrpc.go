// Package converter also holds per-protocol converters. This file converts
// between the kratos-gRPC pb types and the user application DTO.
package converter

import (
	"GS_PROJECT_MODULE/idl/kratos-grpc/pb"
	"GS_PROJECT_MODULE/internal/application/user/dto"
)

// FromKratosGrpcRegisterUserReq converts a kratosgrpc pb RegisterUserReq to an
// application DTO.
func FromKratosGrpcRegisterUserReq(req *pb.RegisterUserReq) *dto.CreateUserReq {
	return &dto.CreateUserReq{
		ID:    req.GetId(),
		Name:  req.GetName(),
		Email: req.GetEmail(),
	}
}

// ToKratosGrpcUser converts a user DTO and request-specific email to a
// kratosgrpc pb User.
func ToKratosGrpcUser(user *dto.UserDTO, email string) *pb.User {
	return &pb.User{
		Id:    user.ID,
		Name:  user.Name,
		Email: email,
	}
}
