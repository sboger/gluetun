package webui

import (
	"context"
	"math"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

const testAPIKey = "secret"

type stubLogger struct{}

func (stubLogger) Info(string)  {}
func (stubLogger) Warn(string)  {}
func (stubLogger) Error(string) {}

func freePort(t *testing.T) *uint16 {
	t.Helper()
	lc := net.ListenConfig{}
	l, err := lc.Listen(context.Background(), "tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("reserving free port: %v", err)
	}
	defer l.Close()
	tcpAddr, ok := l.Addr().(*net.TCPAddr)
	if !ok {
		t.Fatalf("unexpected address type %T", l.Addr())
	}
	if tcpAddr.Port < 0 || tcpAddr.Port > math.MaxUint16 {
		t.Fatalf("port %d out of range", tcpAddr.Port)
	}
	uintPort := uint16(tcpAddr.Port) //nolint:gosec // bounded by MaxUint16 above
	return &uintPort
}

func get(t *testing.T, url string) *http.Response {
	t.Helper()
	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, url, nil)
	if err != nil {
		t.Fatalf("building request: %v", err)
	}
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("requesting %s: %v", url, err)
	}
	return res
}

func TestServerProxiesToControlServer(t *testing.T) {
	t.Parallel()

	var gotAPIKey string
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAPIKey = r.Header.Get("X-API-Key")
		_, _ = w.Write([]byte(`{"version":"test"}`))
	}))
	defer backend.Close()

	backendAddr := strings.TrimPrefix(backend.URL, "http://")
	port := freePort(t)

	srv, err := New(Settings{
		Port:                 port,
		ControlServerAddress: backendAddr,
		APIKey:               testAPIKey,
		Logger:               stubLogger{},
	})
	if err != nil {
		t.Fatalf("creating server: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	ready := make(chan struct{})
	done := make(chan struct{})
	go srv.Run(ctx, ready, done)
	select {
	case <-ready:
	case <-time.After(5 * time.Second):
		t.Fatal("server did not become ready")
	}

	addr := srv.GetAddress()

	res := get(t, "http://"+addr+"/")
	_ = res.Body.Close()
	if res.StatusCode != http.StatusOK {
		t.Fatalf("expected index status 200, got %d", res.StatusCode)
	}

	res = get(t, "http://"+addr+"/v1/version")
	_ = res.Body.Close()
	if res.StatusCode != http.StatusOK {
		t.Fatalf("expected version status 200, got %d", res.StatusCode)
	}
	if gotAPIKey != testAPIKey {
		t.Fatalf("expected API key injected, got %q", gotAPIKey)
	}

	cancel()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("server did not shut down")
	}
}
