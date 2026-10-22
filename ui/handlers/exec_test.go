package handlers

import (
	"testing"

	"github.com/hashicorp/go-hclog"
)

func TestExecHandlerUsesDefaultSameOriginPolicy(t *testing.T) {
	handler := NewExecHandler(NewHandler(hclog.NewNullLogger()))
	if handler.upgrader.CheckOrigin != nil {
		t.Fatal("exec WebSocket must use Gorilla's default same-origin policy")
	}
}
