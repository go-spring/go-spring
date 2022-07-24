package main

import (
	"fmt"

	"github.com/go-spring/spring-base/util"
	"github.com/go-spring/spring-core/gs"
	"github.com/kataras/iris/v12"
)

//---------------- Controller -----------------------------//

type Controller struct {
	HelloController
}

type HelloController struct {
	Service *HelloService `autowire:""`
}

func (c *HelloController) Hello(ctx iris.Context) {
	s := c.Service.Hello(ctx.URLParam("name"))
	ctx.Text(s)
}

//---------------- Service -------------------------------//

type HelloService struct {
}

func (s *HelloService) Hello(name string) string {
	return "hello " + name + "!"
}

//---------------- Engine --------------------------------//

type Engine struct {
	App        *iris.Application
	Address    string        `value:"${http.addr:=:8080}"`
	Controller *Controller   `autowire:""`
	Exit       chan struct{} `autowire:""`
}

func (e *Engine) Init() {
	e.App = iris.Default()
	e.App.Get("/hello", e.Controller.Hello)
	go func() {
		err := e.App.Listen(e.Address)
		fmt.Println(err)
		e.Exit <- struct{}{}
	}()
}

//---------------- main ---------------------------------//

func main() {
	exit := make(chan struct{})
	c := gs.New()
	c.Object(exit)
	c.Object(new(Controller))
	c.Object(new(HelloService))
	c.Object(new(Engine)).Init((*Engine).Init)
	err := c.Refresh()
	util.Panic(err).When(err != nil)
	<-exit
	c.Close()
}

// ➜  ~ curl "http://localhost:8080/hello?name=gopher"
// hello gopher!
