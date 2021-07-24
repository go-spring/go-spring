module github.com/go-spring/spring-core

go 1.14

require (
	github.com/magiconair/properties v1.8.1
	github.com/pelletier/go-toml v1.2.0
	gopkg.in/yaml.v2 v2.2.4
	github.com/go-spring/spring-stl v0.0.0-20210724145437-4e7cb5d3e0ce
)

replace (
	github.com/go-spring/spring-stl => ../spring-stl
)