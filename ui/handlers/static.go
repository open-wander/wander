package handlers

import (
	"io"
	"io/fs"
	"mime"
	"net/http"
	"path"
	"path/filepath"

	uihtmx "github.com/open-wander/wander/ui"
)

// StaticHandler serves embedded static files (CSS, JS, icons) from the static directory.
// Files are embedded into the binary at build time using go:embed.
//
// This handler:
// - Serves files from the embedded filesystem
// - Sets appropriate Content-Type headers based on file extension
// - Sets cache headers for better performance in production
// - Returns 404 for missing files
type StaticHandler struct {
	handler *Handler
	fileSystem http.FileSystem
}

// NewStaticHandler creates a new static file handler.
// It wraps the embedded filesystem for serving via HTTP.
func NewStaticHandler(h *Handler) *StaticHandler {
	// Get the static subdirectory from the embedded filesystem
	staticSubFS, err := fs.Sub(uihtmx.StaticFS, "static")
	if err != nil {
		h.logger.Error("failed to create static filesystem", "error", err)
		// Fallback to the full embedded FS if sub fails
		staticSubFS = uihtmx.StaticFS
	}

	return &StaticHandler{
		handler: h,
		fileSystem: http.FS(staticSubFS),
	}
}

// ServeHTTP handles HTTP requests for static files.
// It strips the /ui/static/ prefix and serves files from the embedded filesystem.
func (s *StaticHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// Only allow GET and HEAD methods
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Get the requested file path
	// Path comes in as /ui/static/css/output.css
	// We need to strip /ui/static/ to get css/output.css
	filePath := r.URL.Path

	// Clean the path to prevent directory traversal
	filePath = path.Clean(filePath)

	// Log the request
	s.handler.logger.Debug("static file request", "path", filePath)

	// Try to open the file
	file, err := s.fileSystem.Open(filePath)
	if err != nil {
		s.handler.logger.Debug("static file not found", "path", filePath, "error", err)
		http.NotFound(w, r)
		return
	}
	defer file.Close()

	// Get file info for modification time and size
	stat, err := file.Stat()
	if err != nil {
		s.handler.logger.Error("failed to stat file", "path", filePath, "error", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	// Don't serve directories
	if stat.IsDir() {
		http.NotFound(w, r)
		return
	}

	// Set Content-Type based on file extension
	ext := filepath.Ext(filePath)
	contentType := mime.TypeByExtension(ext)
	if contentType == "" {
		// Default to octet-stream if we can't determine the type
		contentType = "application/octet-stream"
	}
	w.Header().Set("Content-Type", contentType)

	// Set cache headers for static assets
	// CSS and JS files should be cached for 1 hour in development,
	// longer in production (this can be configured later)
	w.Header().Set("Cache-Control", "public, max-age=3600")

	// Serve the file content
	http.ServeContent(w, r, stat.Name(), stat.ModTime(), file.(io.ReadSeeker))
}

// StripPrefix creates a handler that strips the given prefix from the request path
// before passing it to the static handler. This is useful for serving files under
// a specific URL prefix like /ui/static/.
func (s *StaticHandler) StripPrefix(prefix string) http.Handler {
	return http.StripPrefix(prefix, s)
}
