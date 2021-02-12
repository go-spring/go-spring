package core

import (
	"testing"

	"github.com/go-spring/spring-core/util"
)

func TestParseSingletonTag(t *testing.T) {

	data := map[string]SingletonTag{
		"[]":     {"", "[]", false},
		"[]?":    {"", "[]", true},
		"i":      {"", "i", false},
		"i?":     {"", "i", true},
		":i":     {"", "i", false},
		":i?":    {"", "i", true},
		"int:i":  {"int", "i", false},
		"int:i?": {"int", "i", true},
		"int:":   {"int", "", false},
		"int:?":  {"int", "", true},
	}

	for k, v := range data {
		tag := parseSingletonTag(k)
		util.AssertEqual(t, tag, v)
	}
}

func TestParseBeanTag(t *testing.T) {

	data := map[string]collectionTag{
		"[]":  {[]SingletonTag{}, false},
		"[]?": {[]SingletonTag{}, true},
	}

	for k, v := range data {
		tag := ParseCollectionTag(k)
		util.AssertEqual(t, tag, v)
	}
}
