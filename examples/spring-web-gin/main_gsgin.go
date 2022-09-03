package main

import (
	"github.com/go-spring/spring-core/gs"
	"github.com/go-spring/spring-core/web"
	SpringGin "github.com/go-spring/spring-gin"
)

func main() {
	// example: gzip compress

	gs.Property("web.server.port", 8888)
	gs.Property("web.server.compress.level", 5)
	//	gs.Property("web.server.compress.level", -1)

	gs.Provide(func(cfg web.ServerConfig) web.Server {
		return SpringGin.New(cfg)
	}, "${web.server}")

	gs.GetMapping("/test", func(ctx web.Context) {
		ctx.JSONBlob([]byte(`{"test":1}`))
	})

	err := gs.Run()
	if err != nil {
		panic(err)
	}
}
