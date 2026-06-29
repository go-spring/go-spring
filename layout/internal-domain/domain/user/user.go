// Package user defines the core domain model for the user domain.
package user

// User is the core domain entity for a user.
type User struct {
	ID    int64
	Name  string
	Email string
}

// NewUser creates a new user with the given name and email.
func NewUser(name, email string) *User {
	return &User{
		Name:  name,
		Email: email,
	}
}
