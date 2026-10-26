package handlers

import (
	"mime"
	"testing"
)

func TestDownloadContentDispositionEncodesFilename(t *testing.T) {
	header := downloadContentDisposition(`report\"; filename=unsafe.html`)
	mediaType, params, err := mime.ParseMediaType(header)
	if err != nil {
		t.Fatalf("parse content disposition: %v", err)
	}
	if mediaType != "attachment" {
		t.Fatalf("media type = %q, want attachment", mediaType)
	}
	if got := params["filename"]; got != `report\"; filename=unsafe.html` {
		t.Fatalf("filename = %q", got)
	}
}
