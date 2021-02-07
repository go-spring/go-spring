package SpringSwagger

import (
	"testing"

	"github.com/go-spring/spring-web"
)

func Test_Doc(t *testing.T) {
	s := Doc(nil).
		WithID("go-spring").
		WithHost("https://go-spring.com")
	mapper := SpringWeb.NewMapper(SpringWeb.MethodGet, "/idx", SpringWeb.FUNC(func(context SpringWeb.WebContext) {
		context.String(200, "h")
	}), nil)
	Path("go-spring-idx", *mapper).WithDescription("welcome to go-spring")
	t.Log(s.ReadDoc())
}
