package qbitapi

import (
	"context"
	"io"
	"net"
	"net/http"
	"strings"
	"testing"
	"time"
)

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
	port := uint16(tcpAddr.Port) //nolint:gosec // bounded to uint16 by TCP port range
	return &port
}

func TestServerServesQbitAPI(t *testing.T) {
	t.Parallel()

	stub := &stubBittorrent{}
	srv, err := New(Settings{
		Port:       freePort(t),
		SavePath:   testSavePath,
		Bittorrent: stub,
		Logger:     stubLogger{},
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

	// /app/webapiVersion is what Sonarr/Radarr probe first.
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "http://"+addr+"/api/v2/app/webapiVersion", nil)
	if err != nil {
		t.Fatalf("building request: %v", err)
	}
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("requesting webapiVersion: %v", err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", res.StatusCode)
	}
	body, _ := io.ReadAll(res.Body)
	if strings.TrimSpace(string(body)) != qbitWebAPIVersion {
		t.Fatalf("expected %q, got %q", qbitWebAPIVersion, body)
	}

	// Empty torrent list must serialize as a JSON array.
	req, err = http.NewRequestWithContext(ctx, http.MethodGet, "http://"+addr+"/api/v2/torrents/info", nil)
	if err != nil {
		t.Fatalf("building request: %v", err)
	}
	res, err = http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("requesting torrents info: %v", err)
	}
	defer res.Body.Close()
	body, _ = io.ReadAll(res.Body)
	if strings.TrimSpace(string(body)) != "[]" {
		t.Fatalf("expected empty array, got %q", body)
	}

	cancel()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("server did not shut down")
	}
}
