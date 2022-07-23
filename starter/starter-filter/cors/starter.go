package cors

import (
	"github.com/go-spring/spring-core/gs"
	"github.com/go-spring/spring-core/web"
	"github.com/go-spring/spring-core/web/cors"
)

func init() {
	gs.Provide(cors.New, "${cors}").Export((*web.Filter)(nil))
}
