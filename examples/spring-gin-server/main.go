package main

import (
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/go-spring/spring-base/log"
	"github.com/go-spring/spring-base/util"
	"github.com/go-spring/spring-core/gs"
)

//---------------- Controller -----------------------------//

type Controller struct {
	HelloController
}

type HelloController struct {
	Service *HelloService `autowire:""`
}

func (c *HelloController) Hello(ctx *gin.Context) {
	s := c.Service.Hello(ctx.Query("name"))
	ctx.String(http.StatusOK, s)
}

//---------------- Service -------------------------------//

type HelloService struct {
}

func (s *HelloService) Hello(name string) string {
	return "hello " + name + "!"
}

//---------------- Engine --------------------------------//

type Engine struct {
	Engine     *gin.Engine
	Address    string        `value:"${http.addr:=:8080}"`
	Controller *Controller   `autowire:""`
	Exit       chan struct{} `autowire:""`
}

func (e *Engine) Init() {
	e.Engine = gin.Default()
	e.Engine.GET("/hello", e.Controller.Hello)
	go func() {
		err := e.Engine.Run(e.Address)
		fmt.Println(err)
		e.Exit <- struct{}{}
	}()
}

//---------------- main ---------------------------------//

func main() {

	config := `
		<?xml version="1.0" encoding="UTF-8"?>
		<Configuration/>
	`
	err := log.RefreshBuffer(config, ".xml")
	util.Panic(err).When(err != nil)

	exit := make(chan struct{})
	c := gs.New()
	c.Object(exit)
	c.Object(new(Controller))
	c.Object(new(HelloService))
	c.Object(new(Engine)).Init((*Engine).Init)
	err = c.Refresh()
	util.Panic(err).When(err != nil)
	<-exit
	c.Close()
}

// ➜  ~ curl "http://localhost:8080/hello?name=gopher"
// hello gopher!
