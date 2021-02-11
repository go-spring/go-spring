package swagger_test

import (
	"fmt"
	"testing"

	"github.com/go-spring/spring-core/web"
	"github.com/go-spring/spring-swag/swagger"
)

func Test_Doc(t *testing.T) {
	c := web.NewAbstractContainer(web.ContainerConfig{})
	swagger.Doc(c).WithID("go-spring").WithHost("https://go-spring.com")
	m := c.HandleGet("/idx", web.FUNC(func(ctx web.Context) {}))
	swagger.Path(m).WithDescription("welcome to go-spring")
	web.RegisterSwaggerHandler(func(router web.RootRouter, doc string) { fmt.Println(doc) })
	_ = c.Start()
}
