package main

import (
	"embed"
	"fmt"
	"net/http"

	"github.com/go-spring/spring-core/web"
	"github.com/go-spring/spring-gin"
)

//go:embed hello.html
var htmlFS embed.FS

func main() {
	c := SpringGin.New(web.ServerConfig{Port: 8080, BasePath: "/v1"})
	c.StaticFS("/static", http.FS(htmlFS))
	fmt.Println(c.Start())
}
