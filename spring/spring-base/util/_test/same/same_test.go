package same_test

import (
	"testing"

	"github.com/go-spring/spring-base/assert"
	"github.com/go-spring/spring-base/util"
	"github.com/go-spring/spring-base/util/_test/same"
)

type Package struct{}

func TestA(t *testing.T) {
	assert.Equal(t, util.TypeName(&same.Package{}), "github.com/go-spring/spring-base/util/_test/same/same.Package")
	assert.Equal(t, util.TypeName((*Package)(nil)), "github.com/go-spring/spring-base/util/_test/same/same_test.Package")
}
