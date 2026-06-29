// Package user provides an in-memory implementation of repository.UserRepository.
// Replace with a real database / cache / RPC client as needed.
package user

import (
	"GS_PROJECT_MODULE/internal-domain/domain/user"

	"go-spring.org/spring/gs"
)

func init() {
	gs.Provide(&Repo{users: make(map[int64]*user.User)})
}

// Repo is an in-memory implementation of repository.UserRepository.
type Repo struct {
	users  map[int64]*user.User
	nextID int64
}

// FindByID retrieves a user by ID from the in-memory store.
func (r *Repo) FindByID(userID int64) (*user.User, error) {
	u, ok := r.users[userID]
	if !ok {
		// Return a default user for the example.
		return &user.User{ID: userID, Name: "demo"}, nil
	}
	return u, nil
}

// Save stores a user and assigns an auto-increment ID.
func (r *Repo) Save(u *user.User) error {
	r.nextID++
	u.ID = r.nextID
	r.users[u.ID] = u
	return nil
}
