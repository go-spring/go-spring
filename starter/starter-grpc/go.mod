module github.com/go-spring/starter-grpc

go 1.14

require (
	github.com/go-spring/spring-core v1.0.6-0.20210731095216-fc5849f3eee0
	github.com/go-spring/spring-stl v0.0.0-20210726122404-abcf52621c2c
	github.com/go-spring/starter-core v0.0.0-20210801005940-083fbb1c8f0b
	golang.org/x/lint v0.0.0-20190313153728-d0100b6bd8b3 // indirect
	golang.org/x/tools v0.0.0-20190524140312-2c0ae7006135 // indirect
	google.golang.org/grpc v1.31.0
	honnef.co/go/tools v0.0.0-20190523083050-ea95bdfd59fc // indirect
)

//replace (
//	github.com/go-spring/spring-core => ../../spring/spring-core
//	github.com/go-spring/spring-stl => ../../spring/spring-stl
//	github.com/go-spring/starter-core => ../starter-core
//)
