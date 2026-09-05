module go-spring.org/starter-oauth2-resource-server/example

go 1.26.1

require (
	github.com/golang-jwt/jwt/v5 v5.3.1
	go-spring.org/cloud v0.0.0
	go-spring.org/log v0.1.4
	go-spring.org/spring v1.3.4
)

require (
	github.com/antlr4-go/antlr/v4 v4.13.1 // indirect
	github.com/bytedance/mockey v1.4.6 // indirect
	github.com/expr-lang/expr v1.17.8 // indirect
	github.com/gopherjs/gopherjs v1.20.2 // indirect
	github.com/jtolds/gls v4.20.0+incompatible // indirect
	github.com/magiconair/properties v1.8.10 // indirect
	github.com/pelletier/go-toml v1.9.5 // indirect
	github.com/smarty/assertions v1.16.0 // indirect
	github.com/smartystreets/goconvey v1.8.1 // indirect
	github.com/spf13/cast v1.10.0 // indirect
	go-spring.org/gs-mock v0.0.9 // indirect
	go-spring.org/stdlib v0.1.7 // indirect
	golang.org/x/arch v0.26.0 // indirect
	golang.org/x/exp v0.0.0-20260410095643-746e56fc9e2f // indirect
	golang.org/x/sys v0.47.0 // indirect
	gopkg.in/yaml.v2 v2.4.0 // indirect
)

replace go-spring.org/cloud => ../../../../cloud

replace go-spring.org/starter-oauth2-resource-server => ../

require (
	go-spring.org/starter-http-server v0.0.0
	go-spring.org/starter-oauth2-resource-server v0.0.0-00010101000000-000000000000
)

replace go-spring.org/starter-http-server => ../../../starter-http-server
