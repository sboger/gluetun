package webui

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestHandlerServesIndexHTML(t *testing.T) {
	t.Parallel()

	handler := newHandler(nil)

	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, rec.Code)
	}
	body := rec.Body.String()
	if !strings.Contains(body, "Gluetun Web UI") {
		t.Fatalf("expected index page content, got: %s", body)
	}
	if ct := rec.Header().Get("Content-Type"); !strings.HasPrefix(ct, "text/html") {
		t.Fatalf("expected html content type, got %q", ct)
	}
}

func TestHandlerProxiesV1Requests(t *testing.T) {
	t.Parallel()

	var gotAPIKey string
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAPIKey = r.Header.Get("X-API-Key")
		_, _ = io.WriteString(w, `{"ok":true}`)
	}))
	defer backend.Close()

	target := strings.TrimPrefix(backend.URL, "http://")
	proxy, err := newReverseProxy(target, testAPIKey)
	if err != nil {
		t.Fatalf("creating proxy: %v", err)
	}
	handler := newHandler(proxy)

	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/v1/vpn/status", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, rec.Code)
	}
	if gotAPIKey != testAPIKey {
		t.Fatalf("expected API key to be injected, got %q", gotAPIKey)
	}
	if body := rec.Body.String(); body != `{"ok":true}` {
		t.Fatalf("expected proxied body, got %q", body)
	}
}

func TestNewReverseProxyParsesBarePortAddress(t *testing.T) {
	t.Parallel()

	proxy, err := newReverseProxy(":8000", "")
	if err != nil {
		t.Fatalf("creating proxy: %v", err)
	}
	if proxy == nil {
		t.Fatal("expected non-nil proxy")
	}
}
