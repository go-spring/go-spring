// Package assembler converts between domain entities and application DTOs for the user domain.
package assembler

import (
	"GS_PROJECT_MODULE/internal/application/user/dto"
	"GS_PROJECT_MODULE/internal/domain/user"
)

// ToEntity converts a create request to a domain User entity.
func ToEntity(req *dto.CreateUserReq) *user.User {
	return user.NewUser(req.Name, req.Email)
}

// ToDTO converts a domain User entity to a response DTO.
func ToDTO(user *user.User) *dto.UserDTO {
	return &dto.UserDTO{
		ID:    user.ID,
		Name:  user.Name,
		Email: user.Email,
	}
}