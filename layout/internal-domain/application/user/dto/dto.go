// Package dto defines user request/response models used by the application layer.
package dto

// CreateUserReq is the request DTO for registering a new user.
type CreateUserReq struct {
	ID    string
	Name  string
	Email string
}

// UserDTO is the response DTO for a user.
type UserDTO struct {
	ID    int64
	Name  string
	Email string
}
