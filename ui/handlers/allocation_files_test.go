package handlers

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
)

func TestAllocationFilePreviewHeadTailAndCompleteDownload(t *testing.T) {
	content := []byte("HEAD-MARKER\n" + strings.Repeat("middle\n", 18000) + "TAIL-MARKER\n")
	handler := newAllocationFileTestHandler(t, content, "text/plain")

	for _, preview := range []struct {
		mode, included, excluded string
	}{
		{mode: "head", included: "HEAD-MARKER", excluded: "TAIL-MARKER"},
		{mode: "tail", included: "TAIL-MARKER", excluded: "HEAD-MARKER"},
	} {
		response := runAllocationFileAction(t, handler, "preview", preview.mode)
		assertAllocationFileResponse(t, response, http.StatusOK, []string{preview.included}, []string{preview.excluded})
	}

	download := runAllocationFileAction(t, handler, "download", "")
	if download.Code != http.StatusOK || !bytes.Equal(download.Body.Bytes(), content) {
		t.Fatalf("download length=%d status=%d, want complete %d-byte file", download.Body.Len(), download.Code, len(content))
	}
}

func TestAllocationFileImageAndUnknownBinaryPreview(t *testing.T) {
	imageBytes := []byte("\x89PNG\r\n\x1a\npreview")
	imageHandler := newAllocationFileTestHandler(t, imageBytes, "image/png")

	preview := runAllocationFileAction(t, imageHandler, "preview", "")
	assertAllocationFileResponse(t, preview, http.StatusOK, []string{"data-file-image-preview", "/files/content?"}, nil)

	content := runAllocationFileAction(t, imageHandler, "content", "")
	if content.Code != http.StatusOK || !bytes.Equal(content.Body.Bytes(), imageBytes) {
		t.Fatalf("image content status=%d body=%q", content.Code, content.Body.Bytes())
	}
	if content.Header().Get("Content-Type") != "image/png" || content.Header().Get("X-Content-Type-Options") != "nosniff" {
		t.Fatalf("unsafe image response headers: %#v", content.Header())
	}

	binaryHandler := newAllocationFileTestHandler(t, []byte{0, 1, 2}, "application/octet-stream")
	binary := runAllocationFileAction(t, binaryHandler, "preview", "")
	assertAllocationFileResponse(t, binary, http.StatusOK, []string{"Preview not available for binary files"}, []string{"data-file-image-preview"})
	binaryStream := runAllocationFileAction(t, binaryHandler, "stream", "")
	if binaryStream.Code != http.StatusUnsupportedMediaType {
		t.Fatalf("binary stream status=%d, want %d", binaryStream.Code, http.StatusUnsupportedMediaType)
	}
}

func TestAllocationFilePreviewRejectsInvalidMode(t *testing.T) {
	handler := &AllocationsHandler{}
	response := httptest.NewRecorder()
	handler.FilesPreviewHandler(response, httptest.NewRequest(http.MethodGet, "/ui/allocations/alloc-1/files/preview?path=file.txt&mode=middle", nil), "alloc-1")
	if response.Code != http.StatusBadRequest {
		t.Fatalf("invalid preview mode status=%d, want %d", response.Code, http.StatusBadRequest)
	}
}

func TestAllocationFileEmptyDownloadIsNotMissing(t *testing.T) {
	handler := newAllocationFileTestHandler(t, nil, "text/plain")

	response := runAllocationFileAction(t, handler, "download", "")
	if response.Code != http.StatusOK || response.Body.Len() != 0 {
		t.Fatalf("empty download status=%d length=%d headers=%#v", response.Code, response.Body.Len(), response.Header())
	}
}

func TestAllocationFilePathScopesTaskAndRejectsTraversal(t *testing.T) {
	tests := []struct {
		name    string
		task    string
		path    string
		want    string
		wantErr bool
	}{
		{name: "allocation root", path: "/", want: "/"},
		{name: "allocation special characters", path: "/dir name/file #1.txt", want: "/dir name/file #1.txt"},
		{name: "task root", task: "fixture", path: "/", want: "fixture/"},
		{name: "task file", task: "fixture", path: "/local/file.txt", want: "fixture/local/file.txt"},
		{name: "parent traversal", task: "fixture", path: "/local/../secrets/file", wantErr: true},
		{name: "invalid task", task: "../fixture", path: "/local/file.txt", wantErr: true},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, err := allocationFilePath(test.task, test.path)
			if test.wantErr {
				if err == nil {
					t.Fatalf("allocationFilePath(%q, %q)=%q, want error", test.task, test.path, got)
				}
				return
			}
			if err != nil || got != test.want {
				t.Fatalf("allocationFilePath(%q, %q)=(%q, %v), want (%q, nil)", test.task, test.path, got, err, test.want)
			}
		})
	}
}

func TestAllocationFileListUsesTaskScopedPath(t *testing.T) {
	handler := newAllocationFileTestHandler(t, nil, "text/plain")
	request := requestWithToken(http.MethodGet, "/ui/allocations/alloc-1/files/list?namespace=team-a&region=west")

	files, err := handler.getFiles(request, "alloc-1", "fixture", "/local", "browser-token")
	if err != nil {
		t.Fatalf("getFiles returned error: %v", err)
	}
	if len(files) != 1 || files[0].Name != "file #1.txt" || files[0].Path != "/local/file #1.txt" {
		t.Fatalf("task-scoped files=%#v", files)
	}
}

func TestAllocationFileReadErrorsPreserveStatus(t *testing.T) {
	tests := []struct {
		name       string
		statStatus int
		readStatus int
		isDir      bool
		wantStatus int
	}{
		{name: "vanished after stat", statStatus: http.StatusOK, readStatus: http.StatusNotFound, wantStatus: http.StatusNotFound},
		{name: "permission denied", statStatus: http.StatusForbidden, wantStatus: http.StatusForbidden},
		{name: "directory requested as file", statStatus: http.StatusOK, isDir: true, wantStatus: http.StatusBadRequest},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			backend := newAllocationFileFailureBackend(t, test.statStatus, test.readStatus, test.isDir)
			defer backend.Close()
			handler := NewAllocationsHandler(newJobsTestHandler(t, backend.URL))

			response := runAllocationFileAction(t, handler, "preview", "head")
			if response.Code != test.wantStatus {
				t.Fatalf("preview status=%d body=%q, want %d", response.Code, response.Body.String(), test.wantStatus)
			}
		})
	}
}

func TestAllocationFileStreamReportsMidstreamFailure(t *testing.T) {
	backend := newAllocationFileStreamFailureBackend(t)
	defer backend.Close()
	handler := NewAllocationsHandler(newJobsTestHandler(t, backend.URL))

	response := runAllocationFileAction(t, handler, "stream", "")
	if response.Code != http.StatusOK {
		t.Fatalf("stream status=%d body=%q", response.Code, response.Body.String())
	}
	body := response.Body.String()
	if !strings.Contains(body, "event: file") || !strings.Contains(body, "event: error") {
		t.Fatalf("stream did not preserve data and error events: %q", body)
	}
}

func newAllocationFileTestHandler(t *testing.T, content []byte, contentType string) *AllocationsHandler {
	t.Helper()
	backend := newAllocationFileBackend(t, content, contentType)
	t.Cleanup(backend.Close)
	return NewAllocationsHandler(newJobsTestHandler(t, backend.URL))
}

func runAllocationFileAction(t *testing.T, handler *AllocationsHandler, action, mode string) *httptest.ResponseRecorder {
	t.Helper()
	response := httptest.NewRecorder()
	request := requestWithToken(http.MethodGet, scopedFileURL(action, mode))
	switch action {
	case "preview":
		handler.FilesPreviewHandler(response, request, "alloc-1")
	case "content":
		handler.FilesContentHandler(response, request, "alloc-1")
	case "stream":
		handler.FilesStreamHandler(response, request, "alloc-1")
	case "download":
		handler.FilesDownloadHandler(response, request, "alloc-1")
	default:
		t.Fatalf("unsupported test file action %q", action)
	}
	return response
}

func assertAllocationFileResponse(t *testing.T, response *httptest.ResponseRecorder, status int, includes, excludes []string) {
	t.Helper()
	if response.Code != status {
		t.Fatalf("allocation file response status=%d body=%q, want %d", response.Code, response.Body.String(), status)
	}
	for _, expectation := range []struct {
		values  []string
		present bool
	}{{values: includes, present: true}, {values: excludes}} {
		for _, value := range expectation.values {
			if strings.Contains(response.Body.String(), value) != expectation.present {
				t.Fatalf("allocation file response body presence for %q=%t, want %t: %q", value, !expectation.present, expectation.present, response.Body.String())
			}
		}
	}
}

func scopedFileURL(action, mode string) string {
	url := "/ui/allocations/alloc-1/files/" + action + "?namespace=team-a&region=west&task=fixture&path=%2Flocal%2Fsample.txt"
	if mode != "" {
		url += "&mode=" + mode
	}
	return url
}

func newAllocationFileBackend(t *testing.T, content []byte, contentType string) *httptest.Server {
	t.Helper()
	return newAllocationFileConfiguredBackend(t, allocationFileBackendConfig{
		content: content, contentType: contentType, statStatus: http.StatusOK, readStatus: http.StatusOK,
	})
}

type allocationFileBackendConfig struct {
	content       []byte
	contentType   string
	statStatus    int
	readStatus    int
	isDir         bool
	streamFailure bool
}

func newAllocationFileConfiguredBackend(t *testing.T, config allocationFileBackendConfig) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assertScopedBrowserRequest(t, r)
		switch r.URL.Path {
		case "/v1/allocation/alloc-1":
			writeJSON(t, w, json.RawMessage(`{"ID":"alloc-1"}`))
		case "/v1/client/fs/ls/alloc-1":
			assertAllocationFileTestPathValue(t, r, "fixture/local")
			writeJSON(t, w, json.RawMessage(`[{"Name":"file #1.txt","Size":5,"ContentType":"text/plain"}]`))
		case "/v1/client/fs/stat/alloc-1":
			assertAllocationFileTestPath(t, r)
			if writeAllocationFileTestError(w, config.statStatus) {
				return
			}
			writeJSON(t, w, map[string]any{
				"Name": "sample.txt", "Size": len(config.content), "ContentType": config.contentType, "IsDir": config.isDir,
			})
		case "/v1/client/fs/readat/alloc-1":
			assertAllocationFileTestPath(t, r)
			if writeAllocationFileTestError(w, config.readStatus) {
				return
			}
			offset := allocationFileTestQueryInt(t, r, "offset")
			limit := allocationFileTestQueryInt(t, r, "limit")
			end := min(len(config.content), offset+limit)
			_, _ = w.Write(config.content[offset:end])
		case "/v1/client/fs/cat/alloc-1":
			assertAllocationFileTestPath(t, r)
			if writeAllocationFileTestError(w, config.readStatus) {
				return
			}
			_, _ = w.Write(config.content)
		case "/v1/client/fs/stream/alloc-1":
			assertAllocationFileTestPath(t, r)
			if !config.streamFailure {
				http.NotFound(w, r)
				return
			}
			_, _ = fmt.Fprintln(w, `{"Data":"bGl2ZQo="}`)
			_, _ = fmt.Fprintln(w, `{`)
		default:
			http.NotFound(w, r)
		}
	}))
}

func assertAllocationFileTestPath(t *testing.T, r *http.Request) {
	t.Helper()
	assertAllocationFileTestPathValue(t, r, "fixture/local/sample.txt")
}

func assertAllocationFileTestPathValue(t *testing.T, r *http.Request, want string) {
	t.Helper()
	if got := r.URL.Query().Get("path"); got != want {
		t.Fatalf("allocation filesystem path=%q, want %q", got, want)
	}
}

func allocationFileTestQueryInt(t *testing.T, r *http.Request, name string) int {
	t.Helper()
	value, err := strconv.Atoi(r.URL.Query().Get(name))
	if err != nil {
		t.Fatalf("invalid %s: %v", name, err)
	}
	return value
}

func newAllocationFileFailureBackend(t *testing.T, statStatus, readStatus int, isDir bool) *httptest.Server {
	t.Helper()
	return newAllocationFileConfiguredBackend(t, allocationFileBackendConfig{
		content: []byte("file content"), contentType: "text/plain", statStatus: statStatus, readStatus: readStatus, isDir: isDir,
	})
}

func newAllocationFileStreamFailureBackend(t *testing.T) *httptest.Server {
	t.Helper()
	return newAllocationFileConfiguredBackend(t, allocationFileBackendConfig{
		content: []byte("file content"), contentType: "text/plain", statStatus: http.StatusOK, streamFailure: true,
	})
}

func writeAllocationFileTestError(w http.ResponseWriter, status int) bool {
	if status == 0 || status == http.StatusOK {
		return false
	}
	http.Error(w, http.StatusText(status), status)
	return true
}
