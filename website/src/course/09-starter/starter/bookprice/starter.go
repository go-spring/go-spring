package bookprice

import (
	"context"
	"net/http"
	"time"

	"bookman-pro-09/internal/domain"

	"go-spring.org/spring/gs"
)

func init() {
	gs.Provide(NewClient, gs.TagArg("${bookman.price}")).
		Condition(gs.OnProperty("bookman.price.base-url")).
		Export(gs.As[domain.PriceClient]()).
		Destroy(CloseClient)
}

type Config struct {
	BaseURL string        `value:"${base-url}"`
	Timeout time.Duration `value:"${timeout:=500ms}"`
}

type Client struct {
	baseURL string
	client  *http.Client
}

func NewClient(c Config) (*Client, error) {
	return &Client{baseURL: c.BaseURL, client: &http.Client{Timeout: c.Timeout}}, nil
}

func (c *Client) GetPrice(ctx context.Context, isbn string) (float64, error) {
	return 42, nil
}

func CloseClient(c *Client) error {
	return nil
}
