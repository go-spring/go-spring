package same_test

import (
	"testing"

	"github.com/go-spring/spring-base/assert"
	"github.com/go-spring/spring-base/util"
	same "github.com/go-spring/spring-base/util/_test/nonu"
)

type Package struct{}

func TestA(t *testing.T) {
	assert.Equal(t, util.TypeName(&same.Package{}), "github.com/go-spring/spring-base/util/_test/nonu/same.Package")
	assert.Equal(t, util.TypeName((*Package)(nil)), "github.com/go-spring/spring-base/util/_test/nonu/same_test.Package")
}
