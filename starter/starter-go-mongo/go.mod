module github.com/go-spring/starter-go-mongo

go 1.12

require (
	github.com/go-spring/spring-core v1.0.5
	github.com/go-spring/starter-mongo v1.0.5
	go.mongodb.org/mongo-driver v1.4.0
	github.com/go-spring/spring-stl v0.0.0-20210724145437-4e7cb5d3e0ce // indirect
)

replace (
	github.com/go-spring/spring-stl => ../../spring/spring-stl
	github.com/go-spring/spring-core => ../../spring/spring-core
)
