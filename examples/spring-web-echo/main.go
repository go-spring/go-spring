package main

import (
	"embed"
	"fmt"
	"net/http"

	"github.com/go-spring/spring-base/log"
	"github.com/go-spring/spring-base/util"
	"github.com/go-spring/spring-core/web"
	"github.com/go-spring/spring-echo"
)

//go:embed hello.html
var htmlFS embed.FS

func main() {

	config := `
		<?xml version="1.0" encoding="UTF-8"?>
		<Configuration>
			<Appenders>
				<Console name="Console"/>
			</Appenders>
			<Loggers>
				<Root level="info">
					<AppenderRef ref="Console"/>
				</Root>
			</Loggers>
		</Configuration>
	`
	err := log.RefreshBuffer(config, ".xml")
	util.Panic(err).When(err != nil)

	c := SpringEcho.New(web.ServerConfig{Port: 8080, BasePath: "/v1"})
	c.StaticFS("/static", http.FS(htmlFS))
	fmt.Println(c.Start())
}
