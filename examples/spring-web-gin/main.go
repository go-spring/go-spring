package main

import (
	"embed"
	"fmt"
	"github.com/go-spring/spring-base/log"
	"github.com/go-spring/spring-base/util"
	"github.com/go-spring/spring-core/web"
	"github.com/go-spring/spring-gin"
	"net/http"
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

	c := SpringGin.New(web.ServerConfig{Port: 8080, BasePath: "/v1"})
	c.StaticFS("/static", http.FS(htmlFS))


	//c.GetMapping("/a", func(ctx web.Context) {
	//	ctx.JSONBlob([]byte(`{"a":"a"}`))
	//})
	//	c.AddPrefilter(web.DefaultCorsFilter())
	//c.AddPrefilter(web.CorsFilter(web.CorsConfig{
	//	AllowOrigins: []string{"http://127.0.0.1:8080", "http://127.0.0.1:8081"},
	//	AllowCredentials: false,
	//}))

	//go func() {
	//	time.Sleep(3 * time.Second)
	//	send("GET", "http://127.0.0.1:8080/v1/a", nil)
	//	send("GET", "http://127.0.0.1:8080/v1/a", http.Header{
	//		web.HeaderOrigin: []string{"http://127.0.0.1:8080"},
	//	})
	//	send("GET",  "http://127.0.0.1:8080/v1/a", http.Header{
	//		web.HeaderOrigin: []string{"http://127.0.0.1:8081"},
	//	})
	//	send("OPTIONS",  "http://127.0.0.1:8080/v1/a", http.Header{
	//		web.HeaderOrigin: []string{"http://127.0.0.1:8081"},
	//	})
	//}()

	fmt.Println(c.Start())
}

//func send(method string, url string, header http.Header) {
//	req, err := http.NewRequest(method, url, nil)
//	req.Header = header
//	client := &http.Client{Timeout: 5 * time.Second}
//	resp, err := client.Do(req)
//	if err != nil {
//		panic(err)
//	}
//	defer func(Body io.ReadCloser) {
//		_ = Body.Close()
//	}(resp.Body)
//	var buffer [512]byte
//	result := bytes.NewBuffer(nil)
//	for {
//		n, err := resp.Body.Read(buffer[0:])
//		result.Write(buffer[0:n])
//		if err != nil && err == io.EOF {
//			break
//		} else if err != nil {
//			panic(err)
//		}
//	}
//
//	fmt.Println("result: ", result.String())
//	fmt.Println("response header: ", resp.Header)
//}
