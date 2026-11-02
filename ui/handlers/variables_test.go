package handlers

import (
	"net/url"
	"strings"
	"testing"

	"github.com/shoenig/test/must"
)

func TestParseVariableItems(t *testing.T) {
	form := url.Values{
		"key_0":   {" username "},
		"value_0": {" alice "},
		"key_2":   {"region"},
		"value_2": {"eu-central"},
	}

	items, err := parseVariableItems(form)

	must.NoError(t, err)
	must.Eq(t, map[string]string{
		"username": " alice ",
		"region":   "eu-central",
	}, items)
}

func TestParseVariableItemsRejectsAmbiguousRows(t *testing.T) {
	for _, test := range []struct {
		name string
		form url.Values
		want string
	}{
		{name: "blank key", form: url.Values{"key_0": {" "}, "value_0": {"value"}}, want: "cannot be blank"},
		{name: "missing value field", form: url.Values{"key_0": {"key"}}, want: "missing its value field"},
		{name: "duplicate key", form: url.Values{"key_0": {"key"}, "value_0": {"one"}, "key_1": {" key "}, "value_1": {"two"}}, want: "duplicate variable item key"},
	} {
		t.Run(test.name, func(t *testing.T) {
			_, err := parseVariableItems(test.form)
			must.Error(t, err)
			if !strings.Contains(err.Error(), test.want) {
				t.Fatalf("error = %q, want substring %q", err, test.want)
			}
		})
	}
}

func TestParseVariableItemsJSON(t *testing.T) {
	items, err := parseVariableItems(url.Values{
		"items_json": {`{"username":"alice","region":"eu-central"}`},
	})
	must.NoError(t, err)
	must.Eq(t, map[string]string{"username": "alice", "region": "eu-central"}, items)

	for _, raw := range []string{`["value"]`, `{"count":2}`, `{broken`} {
		_, err := parseVariableItems(url.Values{"items_json": {raw}})
		must.Error(t, err)
	}
}

func TestValidateVariableMutation(t *testing.T) {
	validItems := map[string]string{"key": "value"}
	for _, test := range []struct {
		name      string
		path      string
		namespace string
		items     map[string]string
		wantError bool
	}{
		{name: "ordinary path", path: "team/service", namespace: "default", items: validItems},
		{name: "job path", path: "nomad/jobs/example/config", namespace: "default", items: validItems},
		{name: "template path", path: "nomad/job-templates/example", namespace: "default", items: validItems},
		{name: "wildcard namespace", path: "team/service", namespace: "*", items: validItems, wantError: true},
		{name: "invalid characters", path: "team/service.json", namespace: "default", items: validItems, wantError: true},
		{name: "reserved nomad path", path: "nomad/other", namespace: "default", items: validItems, wantError: true},
		{name: "template directory", path: "nomad/job-templates", namespace: "default", items: validItems, wantError: true},
		{name: "empty items", path: "team/service", namespace: "default", items: map[string]string{}, wantError: true},
		{name: "oversized items", path: "team/service", namespace: "default", items: map[string]string{"key": strings.Repeat("x", maxVariableItemsSize)}, wantError: true},
	} {
		t.Run(test.name, func(t *testing.T) {
			err := validateVariableMutation(test.path, test.namespace, test.items)
			if test.wantError && err == nil {
				t.Fatal("expected validation error")
			}
			if !test.wantError && err != nil {
				t.Fatalf("unexpected validation error: %v", err)
			}
		})
	}
}

func TestVariableJobID(t *testing.T) {
	for _, test := range []struct {
		path string
		want string
	}{
		{path: "nomad/jobs/example", want: "example"},
		{path: "nomad/jobs/example/config", want: "example"},
		{path: "nomad/job-templates/example"},
		{path: "team/nomad/jobs/example"},
		{path: "nomad/jobs/"},
	} {
		must.Eq(t, test.want, variableJobID(test.path))
	}
}
