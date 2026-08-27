module go-spring.org/starter-outbox-gorm/example

go 1.26.1

require (
	go-spring.org/log v0.1.4
	go-spring.org/spring v1.3.4
	go-spring.org/starter-outbox-gorm v0.0.0
	gorm.io/driver/sqlite v1.6.0
	gorm.io/gorm v1.31.1
)

require go-spring.org/cloud v0.0.0

require go-spring.org/starter-outbox-gorm v0.0.0

replace (
	go-spring.org/cloud => ../../../../cloud
	go-spring.org/starter-outbox-gorm => ../
)
