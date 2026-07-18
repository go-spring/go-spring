// Package converter also holds per-protocol converters. This file converts
// between the goframe gRPC pb types and the user application DTO.
package converter

import (
	"GS_PROJECT_MODULE/idl/goframe-grpc/pb"
	"GS_PROJECT_MODULE/internal/application/user/dto"
)

// FromGoframeGrpcRegisterUserReq converts a pb RegisterUserReq to an
// application DTO.
func FromGoframeGrpcRegisterUserReq(req *pb.RegisterUserReq) *dto.CreateUserReq {
	return &dto.CreateUserReq{
		ID:    req.GetId(),
		Name:  req.GetName(),
		Email: req.GetEmail(),
	}
}

// ToGoframeGrpcUser converts a user DTO and request-specific email to a pb User.
func ToGoframeGrpcUser(user *dto.UserDTO, email string) *pb.User {
	return &pb.User{
		Id:    user.ID,
		Name:  user.Name,
		Email: email,
	}
}
