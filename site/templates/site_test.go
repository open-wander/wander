package templates

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	commandagent "github.com/open-wander/wander/command/agent"
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

func TestMultiNodeTutorialConfigsParse(t *testing.T) {
	configs := map[string]string{
		"server.hcl":   MultiNodeServerConfig,
		"client-1.hcl": MultiNodeClientOneConfig,
		"client-2.hcl": MultiNodeClientTwoConfig,
	}

	for name, content := range configs {
		t.Run(name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), name)
			if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
				t.Fatal(err)
			}
			if _, err := commandagent.ParseConfigFile(path); err != nil {
				t.Fatalf("parse tutorial configuration: %v", err)
			}
		})
	}
}
