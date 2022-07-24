module github.com/go-spring/examples/spring-iris-server

go 1.14

require (
	github.com/go-spring/spring-base v1.1.0
	github.com/go-spring/spring-core v1.1.0
	github.com/kataras/iris/v12 v12.2.0-alpha8
	github.com/shurcooL/sanitized_anchor_name v1.0.0 // indirect
)

replace (
	github.com/go-spring/spring-base => ../../spring/spring-base
	github.com/go-spring/spring-core => ../../spring/spring-core
)
