// Package user provides a GORM-backed implementation of the user repository,
// with go-redis used as a read-through cache for FindByID.
// Delete this package (and its starter imports) if you don't need a database.
package user

import (
	"context"
	"encoding/json"
	"errors"
	"strconv"
	"time"

	"GS_PROJECT_MODULE/internal/domain/user"

	"github.com/redis/go-redis/v9"
	"go-spring.org/spring/gs"
	"go-spring.org/stdlib/errutil"
	"gorm.io/gorm"
)

// cacheTTL is how long a user stays in the Redis cache.
const cacheTTL = 10 * time.Minute

func init() {
	gs.Provide(&Repo{}).Init((*Repo).migrate)
}

// userPO is the persistence object mapped to the "users" table.
// It is kept separate from the domain entity so storage concerns
// never leak into the domain model.
type userPO struct {
	ID    int64  `gorm:"primaryKey;autoIncrement"`
	Name  string `gorm:"size:128"`
	Email string `gorm:"size:255"`
}

// TableName sets the mapped table name for userPO.
func (userPO) TableName() string { return "users" }

func (po *userPO) toDomain() *user.User {
	return &user.User{ID: po.ID, Name: po.Name, Email: po.Email}
}

func fromDomain(u *user.User) *userPO {
	return &userPO{ID: u.ID, Name: u.Name, Email: u.Email}
}

// Repo is a GORM-backed implementation of the user repository,
// with a Redis read-through cache in front of FindByID.
type Repo struct {
	DB    *gorm.DB      `autowire:""`
	Redis *redis.Client `autowire:""`
}

// migrate ensures the underlying table exists. It runs after dependency injection.
func (r *Repo) migrate() error {
	return r.DB.AutoMigrate(&userPO{})
}

// cacheKey builds the Redis key for a user ID.
func cacheKey(userID int64) string {
	return "user:" + strconv.FormatInt(userID, 10)
}

// FindByID retrieves a user by ID, using Redis as a read-through cache.
func (r *Repo) FindByID(userID int64) (*user.User, error) {
	ctx := context.Background()
	key := cacheKey(userID)

	// 1. Try cache.
	if data, err := r.Redis.Get(ctx, key).Bytes(); err == nil {
		var po userPO
		if json.Unmarshal(data, &po) == nil {
			return po.toDomain(), nil
		}
	}

	// 2. Fall back to the database.
	var po userPO
	if err := r.DB.First(&po, userID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errutil.Explain(nil, "user %d not found", userID)
		}
		return nil, errutil.Stack(err, "find user %d", userID)
	}

	// 3. Populate cache (best-effort).
	if data, err := json.Marshal(po); err == nil {
		_ = r.Redis.Set(ctx, key, data, cacheTTL).Err()
	}
	return po.toDomain(), nil
}

// Save stores a user, assigns an auto-increment ID, and invalidates the cache.
func (r *Repo) Save(u *user.User) error {
	po := fromDomain(u)
	if err := r.DB.Create(po).Error; err != nil {
		return errutil.Stack(err, "save user")
	}
	u.ID = po.ID
	// Invalidate any stale cache entry (best-effort).
	_ = r.Redis.Del(context.Background(), cacheKey(u.ID)).Err()
	return nil
}
