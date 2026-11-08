package agent

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/open-wander/wander/site/examples"
)

func TestSiteMultiNodeConfigsParse(t *testing.T) {
	configs := map[string]string{
		"server.hcl":   examples.MultiNodeServerConfig,
		"client-1.hcl": examples.MultiNodeClientOneConfig,
		"client-2.hcl": examples.MultiNodeClientTwoConfig,
	}

	for name, content := range configs {
		t.Run(name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), name)
			if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
				t.Fatal(err)
			}
			if _, err := ParseConfigFile(path); err != nil {
				t.Fatalf("parse published configuration: %v", err)
			}
		})
	}
}
