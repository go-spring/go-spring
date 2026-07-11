// Package converter also holds per-protocol converters. This file converts
// between the gRPC pb types and the user application DTO; the HTTP converters
// live alongside it in converter.go.
package converter

import (
	"GS_PROJECT_MODULE/idl/grpc/pb"
	"GS_PROJECT_MODULE/internal/application/user/dto"
)

// FromGrpcRegisterUserReq converts a pb RegisterUserReq to an application DTO.
func FromGrpcRegisterUserReq(req *pb.RegisterUserReq) *dto.CreateUserReq {
	return &dto.CreateUserReq{
		ID:    req.GetId(),
		Name:  req.GetName(),
		Email: req.GetEmail(),
	}
}

// ToGrpcUser converts a user DTO and request-specific email to a pb User.
func ToGrpcUser(user *dto.UserDTO, email string) *pb.User {
	return &pb.User{
		Id:    user.ID,
		Name:  user.Name,
		Email: email,
	}
}
