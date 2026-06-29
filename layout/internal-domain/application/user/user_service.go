// Package user provides user-related application services (query, command).
package user

import (
	"GS_PROJECT_MODULE/internal-domain/application/user/assembler"
	"GS_PROJECT_MODULE/internal-domain/application/user/dto"
	userrepo "GS_PROJECT_MODULE/internal-domain/infra/repo/user"

	"go-spring.org/spring/gs"
)

func init() {
	gs.Provide(&UserService{})
}

// UserService provides user query operations for other domains.
type UserService struct {
	UserRepo *userrepo.Repo `autowire:""`
}

// GetUser retrieves a user by ID. Returns error if not found.
func (s *UserService) GetUser(userID int64) (*dto.UserDTO, error) {
	user, err := s.UserRepo.FindByID(userID)
	if err != nil {
		return nil, err
	}
	return assembler.ToDTO(user), nil
}

// RegisterUser creates a new user and returns the registered user DTO.
func (s *UserService) RegisterUser(req *dto.CreateUserReq) (*dto.UserDTO, error) {
	user := assembler.ToEntity(req)
	if err := s.UserRepo.Save(user); err != nil {
		return nil, err
	}
	return assembler.ToDTO(user), nil
}
