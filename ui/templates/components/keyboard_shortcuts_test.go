package components

import (
	"context"
	"strings"
	"testing"
)

func TestKeyboardShortcutsDialogRendersAccessibleControls(t *testing.T) {
	var output strings.Builder
	if err := KeyboardShortcutsDialog().Render(context.Background(), &output); err != nil {
		t.Fatalf("render keyboard shortcuts dialog: %v", err)
	}

	html := output.String()
	for _, expected := range []string{
		`data-keyboard-shortcuts-dialog`,
		`role="dialog"`,
		`aria-modal="true"`,
		`aria-labelledby="keyboard-shortcuts-title"`,
		`data-shortcuts-enabled`,
		`data-shortcuts-reset`,
		`data-shortcut-id="accessControl"`,
		`data-shortcut-id="showHelp"`,
		`aria-live="polite"`,
	} {
		if !strings.Contains(html, expected) {
			t.Errorf("rendered dialog missing %s", expected)
		}
	}
}
