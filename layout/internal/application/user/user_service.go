// Package user provides user-related application services (query, command).
package user

import (
	"GS_PROJECT_MODULE/internal/application/user/assembler"
	"GS_PROJECT_MODULE/internal/application/user/dto"
	userrepo "GS_PROJECT_MODULE/internal/infra/repo/user"

	"go-spring.org/spring/gs"
	"go-spring.org/stdlib/errutil"
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
		return nil, errutil.Stack(err, "GetUser(%d)", userID)
	}
	return assembler.ToDTO(user), nil
}

// RegisterUser creates a new user and returns the registered user DTO.
func (s *UserService) RegisterUser(req *dto.CreateUserReq) (*dto.UserDTO, error) {
	user := assembler.ToEntity(req)
	if err := s.UserRepo.Save(user); err != nil {
		return nil, errutil.Stack(err, "RegisterUser save")
	}
	return assembler.ToDTO(user), nil
}
