package boot

import (
	"github.com/go-spring/spring-core/app"
)

var gApp = app.NewApplication()

func App() *app.Application { return gApp }
