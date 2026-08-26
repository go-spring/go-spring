module go-spring.org/starter-gorm-mysql/example-cloudnative

go 1.26.1

require (
	go-spring.org/cloud v0.0.0
	go-spring.org/spring v1.3.4
	go-spring.org/starter-actuator v0.0.0
	go-spring.org/starter-config-file v0.0.0
	go-spring.org/starter-gorm-mysql v0.0.0
)

require (
	filippo.io/edwards25519 v1.1.0 // indirect
	github.com/antlr4-go/antlr/v4 v4.13.1 // indirect
	github.com/bytedance/mockey v1.4.6 // indirect
	github.com/cespare/xxhash/v2 v2.3.0 // indirect
	github.com/expr-lang/expr v1.17.8 // indirect
	github.com/fsnotify/fsnotify v1.9.0 // indirect
	github.com/go-logr/logr v1.4.4 // indirect
	github.com/go-logr/stdr v1.2.2 // indirect
	github.com/go-sql-driver/mysql v1.9.3 // indirect
	github.com/gopherjs/gopherjs v1.20.2 // indirect
	github.com/jinzhu/inflection v1.0.0 // indirect
	github.com/jinzhu/now v1.1.5 // indirect
	github.com/jtolds/gls v4.20.0+incompatible // indirect
	github.com/magiconair/properties v1.8.10 // indirect
	github.com/pelletier/go-toml v1.9.5 // indirect
	github.com/smarty/assertions v1.16.0 // indirect
	github.com/smartystreets/goconvey v1.8.1 // indirect
	github.com/spf13/cast v1.10.0 // indirect
	go-spring.org/gs-mock v0.0.9 // indirect
	go-spring.org/log v0.1.4 // indirect
	go-spring.org/starter-gorm v0.0.0 // indirect
	go-spring.org/starter-governance v0.0.0
	go-spring.org/stdlib v0.1.7 // indirect
	go.opentelemetry.io/auto/sdk v1.2.1 // indirect
	go.opentelemetry.io/otel v1.45.0 // indirect
	go.opentelemetry.io/otel/metric v1.45.0 // indirect
	go.opentelemetry.io/otel/trace v1.45.0 // indirect
	golang.org/x/arch v0.26.0 // indirect
	golang.org/x/exp v0.0.0-20260410095643-746e56fc9e2f // indirect
	golang.org/x/sys v0.47.0 // indirect
	golang.org/x/text v0.41.0 // indirect
	gopkg.in/yaml.v2 v2.4.0 // indirect
	gorm.io/driver/mysql v1.6.0 // indirect
	gorm.io/gorm v1.31.2 // indirect
)

replace (
	go-spring.org/cloud => ../../../cloud
	go-spring.org/starter-actuator => ../../experimental/starter-actuator
	go-spring.org/starter-config-file => ../../starter-config-file
	go-spring.org/starter-gorm => ../../../starter/starter-gorm
	go-spring.org/starter-gorm-mysql => ..
	go-spring.org/stdlib => ../../../stdlib
)

replace go-spring.org/starter-governance => ../../starter-governance
