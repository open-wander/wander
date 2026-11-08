package templates

import (
	"bytes"
	"context"
	"strings"
	"testing"

	"github.com/open-wander/wander/jobspec2"
)

func TestPagesHaveDocumentLandmarks(t *testing.T) {
	for _, page := range Pages() {
		t.Run(page.Output, func(t *testing.T) {
			var output bytes.Buffer
			if err := page.Component.Render(context.Background(), &output); err != nil {
				t.Fatal(err)
			}
			html := output.String()
			for _, required := range []string{"<!doctype html>", "<title>", `<main id="main">`, "<footer"} {
				if !strings.Contains(strings.ToLower(html), strings.ToLower(required)) {
					t.Errorf("missing %q", required)
				}
			}
		})
	}
}

func TestHelloWanderJobParses(t *testing.T) {
	job, err := jobspec2.Parse("hello-wander.nomad.hcl", strings.NewReader(HelloWanderJob))
	if err != nil {
		t.Fatalf("parse tutorial job: %v", err)
	}
	if job.ID == nil || *job.ID != "hello-wander" {
		t.Fatalf("unexpected job ID: %#v", job.ID)
	}
}
