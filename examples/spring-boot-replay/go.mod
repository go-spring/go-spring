module github.com/go-spring/examples/spring-boot-replay

go 1.14

require (
	github.com/go-spring/spring-base v1.1.0-rc1.0.20211030141542-4d3ac8ed9e93
	github.com/go-spring/spring-core v1.1.0-rc1.0.20211031004208-3997c4df4f37
	github.com/go-spring/starter-echo v1.1.0-rc1.0.20211031024329-1e2e9e4ae2f6
	github.com/go-spring/starter-go-redis v1.1.0-rc1.0.20211031024351-16ddd5333d0d
	github.com/go-spring/starter-web v1.1.0-rc1.0.20211031024817-384f04ad4382 // indirect
)

replace (
	github.com/go-spring/spring-base => ../../spring/spring-base
	github.com/go-spring/spring-core => ../../spring/spring-core
	github.com/go-spring/spring-echo => ../../spring/spring-echo
	github.com/go-spring/spring-go-redis => ../../spring/spring-go-redis
	github.com/go-spring/starter-web => ../../starter/starter-web
	github.com/go-spring/starter-go-redis => ../../starter/starter-go-redis
)
