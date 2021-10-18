module github.com/go-spring/spring-echo

go 1.14

require (
	github.com/go-spring/spring-core v1.0.6-0.20211001040940-f4fed6e6c943
	github.com/go-spring/spring-base v1.1.0-beta.0.20211001035852-bfba805daa15
	github.com/labstack/echo v3.3.10+incompatible
	github.com/labstack/gommon v0.3.0 // indirect
	golang.org/x/crypto v0.0.0-20210616213533-5ff15b29337e // indirect
)

replace (
	github.com/go-spring/spring-core => ../spring-core
	github.com/go-spring/spring-base => ../spring-base
)
