module go-spring.org/starter-http-server

go 1.26.1

require (
	go-spring.org/cloud v0.0.0
	go-spring.org/starter-governance v0.0.0
)

replace go-spring.org/cloud => ../../cloud
replace go-spring.org/starter-governance => ../starter-governance
