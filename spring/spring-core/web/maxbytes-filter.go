package web

import (
	"github.com/go-spring/spring-base/log"
	"net/http"
)

// MaxBytesFilter limit the maximum request contentLength
func MaxBytesFilter(maxBytes int64) *Prefilter {
	return FuncPrefilter(func(ctx Context, chain FilterChain) {
		if maxBytes < 0 {
			chain.Next(ctx)
			return
		}

		contentLength := ctx.Request().ContentLength
		if contentLength > maxBytes {
			log.Status.Errorf("request entity too large, limit is %d, but got %d, rejected with code %d",
				maxBytes, contentLength, http.StatusRequestEntityTooLarge)

			ctx.SetStatus(http.StatusRequestEntityTooLarge)
			return
		}

		chain.Next(ctx)
	})

}
