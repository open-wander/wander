package components

import (
	"testing"

	"github.com/open-wander/wander/ui/tests/testutil"
)

func TestConnectionStatus(t *testing.T) {
	tests := []struct {
		name  string
		props ConnectionStatusProps
		want  []string
	}{
		{
			name: "renders connection status with ID",
			props: ConnectionStatusProps{
				ID: "test-connection-status",
			},
			want: []string{
				`id="test-connection-status"`,
				`x-data="connectionStatus()"`,
				`x-init="init()"`,
			},
		},
		{
			name: "includes status indicator elements",
			props: ConnectionStatusProps{
				ID: "conn-status",
			},
			want: []string{
				`bg-green-500`,  // Connected state
				`bg-red-500`,    // Disconnected state
				`bg-yellow-500`, // Connecting state
				`bg-neutral-400`, // Error state
			},
		},
		{
			name: "includes status text",
			props: ConnectionStatusProps{
				ID: "status",
			},
			want: []string{
				`x-text="statusText"`,
				`text-neutral-600`,
			},
		},
		{
			name: "includes reconnect functionality",
			props: ConnectionStatusProps{
				ID: "status",
			},
			want: []string{
				`@click="reconnect()"`,
				`Reconnect Now`,
			},
		},
		{
			name: "includes mobile tooltip",
			props: ConnectionStatusProps{
				ID: "status",
			},
			want: []string{
				`showTooltip`,
				`@click.away="showTooltip = false"`,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Render the component
			html := testutil.MustRender(t, ConnectionStatus(tt.props))

			// Check for expected content
			for _, want := range tt.want {
				testutil.ContainsHTML(t, html, want)
			}
		})
	}
}

func TestConnectionStatusIntegration(t *testing.T) {
	t.Run("renders without errors", func(t *testing.T) {
		component := ConnectionStatus(ConnectionStatusProps{
			ID: "integration-test",
		})

		html := testutil.MustRender(t, component)

		if len(html) == 0 {
			t.Error("Render() produced empty output")
		}

		// Verify key Alpine.js attributes are present
		testutil.ContainsHTML(t, html, `x-data="connectionStatus()"`)
		testutil.ContainsHTML(t, html, `x-init="init()"`)
	})

	t.Run("includes all connection states", func(t *testing.T) {
		html := testutil.MustRender(t, ConnectionStatus(ConnectionStatusProps{
			ID: "state-test",
		}))

		states := []string{
			"connected",
			"disconnected",
			"connecting",
			"reconnecting",
			"error",
		}

		for _, state := range states {
			testutil.ContainsHTML(t, html, state)
		}
	})

	t.Run("includes responsive classes", func(t *testing.T) {
		html := testutil.MustRender(t, ConnectionStatus(ConnectionStatusProps{
			ID: "responsive-test",
		}))

		responsiveClasses := []string{
			"hidden md:inline-block", // Desktop text
			"md:hidden",              // Mobile button
		}

		for _, class := range responsiveClasses {
			testutil.ContainsHTML(t, html, class)
		}
	})
}

func TestConnectionStatusAccessibility(t *testing.T) {
	t.Run("includes ARIA labels", func(t *testing.T) {
		html := testutil.MustRender(t, ConnectionStatus(ConnectionStatusProps{
			ID: "a11y-test",
		}))

		testutil.ContainsHTML(t, html, `aria-label="Connection status details"`)
	})

	t.Run("includes proper semantic markup", func(t *testing.T) {
		html := testutil.MustRender(t, ConnectionStatus(ConnectionStatusProps{
			ID: "semantic-test",
		}))

		// Should use button elements for interactive controls
		testutil.ContainsElement(t, html, "button")
	})
}
