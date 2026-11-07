// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package components

import "testing"

func TestScopedURL(t *testing.T) {
	for _, test := range []struct {
		name      string
		namespace string
		region    string
		want      string
	}{
		{name: "unscoped", want: "/ui/jobs/example"},
		{name: "namespace", namespace: "team/a", want: "/ui/jobs/example?namespace=team%2Fa"},
		{name: "region", region: "west", want: "/ui/jobs/example?region=west"},
		{name: "both", namespace: "team-a", region: "west", want: "/ui/jobs/example?namespace=team-a&region=west"},
	} {
		t.Run(test.name, func(t *testing.T) {
			if got := ScopedURL("/ui/jobs/example", test.namespace, test.region); got != test.want {
				t.Fatalf("ScopedURL() = %q, want %q", got, test.want)
			}
		})
	}
}
