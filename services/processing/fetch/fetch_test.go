package fetch_test

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"processing/fetch"
)

const sampleHTML = `<!DOCTYPE html>
<html>
<head><title>Test</title></head>
<body>
  <nav>Navigation garbage</nav>
  <article>
    <h1>Real Article Title</h1>
    <p>First paragraph of content.</p>
    <p>Second paragraph of content.</p>
  </article>
  <footer>Footer garbage</footer>
</body>
</html>`

func TestFetchAndExtract(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		w.Write([]byte(sampleHTML))
	}))
	defer srv.Close()

	text, err := fetch.FetchAndExtract(srv.URL)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(text, "First paragraph") {
		t.Errorf("expected article content in output, got: %q", text)
	}
	if strings.Contains(text, "Navigation garbage") {
		t.Errorf("nav content should be stripped, got: %q", text)
	}
	if strings.Contains(text, "Footer garbage") {
		t.Errorf("footer content should be stripped, got: %q", text)
	}
	if len(text) < 10 {
		t.Errorf("extracted text too short: %q", text)
	}
}
