package web

import (
	"github.com/go-spring/spring-base/assert"
	"github.com/go-spring/spring-base/util"
	"testing"
)

func TestConfigAddAllow(t *testing.T) {
	config := CorsConfig{}
	config.AllowAllOrigins = true
	//config.AllowOrigins = []string{"http://a.com"}
	//config.AllowOriginFunc = func(o string) bool {
	//	return true
	//}

	config.AddAllowMethods("POST")
	config.AddAllowMethods("GET", "PUT")

	config.AddAllowHeaders("Some", " cool")
	config.AddAllowHeaders("header")

	config.AddExposeHeaders("exposed", "header")
	config.AddExposeHeaders("hey")

	err := config.Validate()
	util.Panic(err).When(err != nil)

	assert.Equal(t, config.AllowMethods, []string{"POST", "GET", "PUT"})
	assert.Equal(t, config.AllowHeaders, []string{"Some", " cool", "header"})
	assert.Equal(t, config.ExposeHeaders, []string{"exposed", "header", "hey"})
	assert.Equal(t, config.getAllowedSchemas(), []string{"http://", "https://"})
}
