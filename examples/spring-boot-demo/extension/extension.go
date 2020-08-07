package extension

import (
	"github.com/go-spring/go-spring/spring-boot"
)

func init() {
	SpringBoot.RegisterFileConfigReader(".ini", SpringBoot.ViperReadBuffer("ini"))
}
