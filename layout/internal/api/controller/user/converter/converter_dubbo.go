// Package converter also hosts Dubbo-specific converters, isolated in
// converter_dubbo.go to avoid clashing with the HTTP converter names.
package converter

import (
	"GS_PROJECT_MODULE/idl/dubbo/triple"
	"GS_PROJECT_MODULE/internal/application/user/dto"
)

// FromDubboRegisterUserReq converts a triple RegisterUserReq to an application DTO.
func FromDubboRegisterUserReq(req *triple.RegisterUserReq) *dto.CreateUserReq {
	return &dto.CreateUserReq{
		ID:    req.GetId(),
		Name:  req.GetName(),
		Email: req.GetEmail(),
	}
}

// ToDubboUser converts a user DTO and request-specific fields to a triple User model.
func ToDubboUser(user *dto.UserDTO, email string) *triple.User {
	return &triple.User{
		Id:    user.ID,
		Name:  user.Name,
		Email: email,
	}
}
