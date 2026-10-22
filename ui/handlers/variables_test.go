package handlers

import (
	"net/url"
	"testing"

	"github.com/shoenig/test/must"
)

func TestParseVariableItems(t *testing.T) {
	form := url.Values{
		"key_0":   {" username "},
		"value_0": {" alice "},
		"key_1":   {""},
		"value_1": {"ignored"},
		"key_2":   {"region"},
		"value_2": {"eu-central"},
		"key_3":   {"missing-value"},
	}

	items := parseVariableItems(form)

	must.Eq(t, map[string]string{
		"username": " alice ",
		"region":   "eu-central",
	}, items)
}
