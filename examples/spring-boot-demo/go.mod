module github.com/go-spring/examples/spring-boot-demo

go 1.14

require (
	github.com/DATA-DOG/go-sqlmock v1.4.1
	github.com/dgrijalva/jwt-go v3.2.0+incompatible // indirect
	github.com/elliotchance/redismock v1.5.3
	github.com/go-redis/redis v6.15.9+incompatible
	github.com/go-spring/spring-base v1.1.0-beta.0.20211001035852-bfba805daa15
	github.com/go-spring/spring-core v1.0.6-0.20211001040940-f4fed6e6c943
	github.com/go-spring/starter-echo v1.1.0-alpha.0.20211002014844-f5432e77cd0f
	github.com/go-spring/starter-go-redis v1.1.0-alpha.0.20211002011402-f6f9d978d487
	github.com/go-spring/starter-gorm v1.1.0-alpha.0.20211002011216-bf6761cbef69
	github.com/jinzhu/gorm v1.9.15
	github.com/labstack/echo v3.3.10+incompatible
	github.com/spf13/viper v1.3.1
	go.mongodb.org/mongo-driver v1.5.1
)

//replace (
//	github.com/go-spring/spring-core => ../../spring/spring-core
//)
