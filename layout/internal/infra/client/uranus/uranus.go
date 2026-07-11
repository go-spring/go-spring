// Package uranus provides a client for the Uranus platform service (e.g. user center, auth center).
package uranus

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"go-spring.org/spring/gs"
)

func init() {
	gs.Provide(NewUranusClient)
}

// Config holds the Uranus client configuration.
type Config struct {
	BaseURL string `value:"${uranus.base-url:=http://localhost:8080}"`
	Timeout int    `value:"${uranus.timeout:=5000}"`
}

// UranusClient is a client for calling the Uranus platform service.
type UranusClient struct {
	config Config
	client *http.Client
}

// NewUranusClient creates a new UranusClient with the given config.
func NewUranusClient(config Config) *UranusClient {
	return &UranusClient{
		config: config,
		client: &http.Client{Timeout: time.Duration(config.Timeout) * time.Millisecond},
	}
}

// UserInfo represents user data returned by the Uranus platform.
type UserInfo struct {
	ID   int64
	Name string
}

// GetUser fetches user info from the Uranus platform by user ID (currently a mock).
func (c *UranusClient) GetUser(ctx context.Context, id int64) (*UserInfo, error) {
	// TODO: replace with real HTTP/RPC call to Uranus service.
	_ = c.client
	_ = c.config
	return &UserInfo{ID: id, Name: fmt.Sprintf("uranus-user-%d", id)}, nil
}
