package taskgrouppartials

import (
	"bytes"
	"context"
	"strings"
	"testing"
)

func TestScalingControlsUsesCSPComponentData(t *testing.T) {
	var output bytes.Buffer
	if err := ScalingControls("example", TaskGroupDetail{
		Name:  "web",
		Count: 2,
		Scaling: &ScalingConfig{
			Enabled: true,
			Min:     1,
			Max:     5,
		},
	}).Render(context.Background(), &output); err != nil {
		t.Fatalf("render scaling controls: %v", err)
	}

	html := output.String()
	for _, expected := range []string{
		`x-data="scaleControls"`,
		`data-count="2"`,
		`data-min-count="1"`,
		`data-max-count="5"`,
		`data-has-constraints="true"`,
		`@htmx:after-request="handleScaleResponse(event)"`,
	} {
		if !strings.Contains(html, expected) {
			t.Fatalf("rendered scaling controls do not contain %q: %s", expected, html)
		}
	}
	if strings.Contains(html, "originalCount:") {
		t.Fatalf("scaling state must not be embedded in the Alpine expression: %s", html)
	}
}
