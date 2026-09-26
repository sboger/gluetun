package qbitapi

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync"
	"testing"

	"github.com/qdm12/gluetun/internal/models"
)

const (
	testSavePath = "/downloads"
	okResponse   = "Ok."
	hashA        = "aaaa"
)

type stubLogger struct{}

func (stubLogger) Info(string)  {}
func (stubLogger) Warn(string)  {}
func (stubLogger) Error(string) {}

// stubBittorrent is an in-memory stand-in for the embedded client.
type stubBittorrent struct {
	mu         sync.Mutex
	magnets    []string
	files      []string
	categories map[string]string // hash -> category
	torrents   []models.BittorrentTorrent
	removed    []string
}

func (s *stubBittorrent) AddMagnet(magnet string, category string) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.magnets = append(s.magnets, magnet)
	hash := fmt.Sprintf("MAGNET%x", len(s.magnets))
	if s.categories == nil {
		s.categories = map[string]string{}
	}
	s.categories[hash] = category
	s.torrents = append(s.torrents, models.BittorrentTorrent{
		InfoHash: hash,
		Name:     "magnet-dl",
		Category: category,
	})
	return hash, nil
}

func (s *stubBittorrent) AddTorrentFile(name string, _ []byte, category string) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.files = append(s.files, name)
	hash := fmt.Sprintf("FILE%x", len(s.files))
	if s.categories == nil {
		s.categories = map[string]string{}
	}
	s.categories[hash] = category
	s.torrents = append(s.torrents, models.BittorrentTorrent{
		InfoHash: hash,
		Name:     name,
		Category: category,
	})
	return hash, nil
}

func (s *stubBittorrent) ListTorrents() (torrents []models.BittorrentTorrent) {
	s.mu.Lock()
	defer s.mu.Unlock()
	torrents = append(torrents, s.torrents...)
	return torrents
}

func (s *stubBittorrent) RemoveTorrent(hash string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.removed = append(s.removed, hash)
	for i := range s.torrents {
		if s.torrents[i].InfoHash == hash {
			s.torrents = append(s.torrents[:i], s.torrents[i+1:]...)
			break
		}
	}
	return nil
}

func newTestHandler(stub *stubBittorrent) http.Handler {
	return newHandler(handlerSettings{
		savePath:   testSavePath,
		username:   "",
		password:   "",
		dhtEnabled: true,
		bittorrent: stub,
		logger:     stubLogger{},
	})
}

func doRequest(t *testing.T, h http.Handler, method, path string,
	body io.Reader, contentType string,
) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequestWithContext(context.Background(), method, path, body)
	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

func TestAppEndpoints(t *testing.T) {
	t.Parallel()
	h := newTestHandler(&stubBittorrent{})

	cases := map[string]struct {
		path string
		want string
	}{
		"version":       {"/api/v2/app/version", qbitAppVersion},
		"webapiVersion": {"/api/v2/app/webapiVersion", qbitWebAPIVersion},
		"defaultSave":   {"/api/v2/app/defaultSavePath", testSavePath},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			rec := doRequest(t, h, http.MethodGet, tc.path, nil, "")
			if rec.Code != http.StatusOK {
				t.Fatalf("expected 200, got %d", rec.Code)
			}
			got := strings.TrimSpace(rec.Body.String())
			if got != tc.want {
				t.Errorf("expected %q, got %q", tc.want, got)
			}
		})
	}
}

func TestPreferencesDhtEnabled(t *testing.T) {
	t.Parallel()
	h := newTestHandler(&stubBittorrent{})
	rec := doRequest(t, h, http.MethodGet, "/api/v2/app/preferences", nil, "")
	var prefs qbitPreferences
	if err := json.Unmarshal(rec.Body.Bytes(), &prefs); err != nil {
		t.Fatalf("parsing preferences: %v", err)
	}
	if !prefs.DhtEnabled {
		t.Error("expected dht to be enabled")
	}
	if prefs.SavePath != testSavePath {
		t.Errorf("expected save_path %s, got %q", testSavePath, prefs.SavePath)
	}
}

func TestTorrentsInfoStateMapping(t *testing.T) {
	t.Parallel()

	stub := &stubBittorrent{}
	stub.torrents = []models.BittorrentTorrent{
		{ // downloading, metadata known, partial
			InfoHash: hashA, Name: "down", Length: 100, BytesCompleted: 40,
			PercentComplete: 40, State: "downloading", SavePath: testSavePath,
			ContentPath: testSavePath + "/down", Category: "tv",
		},
		{ // complete -> uploading
			InfoHash: "bbbb", Name: "done", Length: 200, BytesCompleted: 200,
			PercentComplete: 100, State: "seeding", SavePath: testSavePath,
			ContentPath: testSavePath + "/done", Downloaded: 200, Uploaded: 50,
		},
		{ // metadata not yet fetched -> metaDL
			InfoHash: "cccc", Name: "meta", Length: 0, BytesCompleted: 0,
			PercentComplete: 0, State: "fetching metadata", SavePath: testSavePath,
			ContentPath: testSavePath + "/meta",
		},
	}

	h := newTestHandler(stub)
	rec := doRequest(t, h, http.MethodGet, "/api/v2/torrents/info", nil, "")
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}

	var infos []qbitTorrentInfo
	if err := json.Unmarshal(rec.Body.Bytes(), &infos); err != nil {
		t.Fatalf("parsing torrents info: %v", err)
	}
	if len(infos) != 3 {
		t.Fatalf("expected 3 torrents, got %d", len(infos))
	}

	if infos[0].State != "downloading" || infos[0].Progress != 0.4 {
		t.Errorf("unexpected downloading torrent: %+v", infos[0])
	}
	if infos[1].State != "uploading" || infos[1].ETA != completedETA {
		t.Errorf("unexpected completed torrent: %+v", infos[1])
	}
	if infos[2].State != "metaDL" {
		t.Errorf("unexpected meta torrent state: %s", infos[2].State)
	}
}

func TestFilterByCategory(t *testing.T) {
	t.Parallel()

	stub := &stubBittorrent{}
	stub.torrents = []models.BittorrentTorrent{
		{InfoHash: hashA, Name: "a", Category: "tv"},
		{InfoHash: "bbbb", Name: "b", Category: "movies"},
	}
	h := newTestHandler(stub)

	rec := doRequest(t, h, http.MethodGet, "/api/v2/torrents/info?category=tv", nil, "")
	var infos []qbitTorrentInfo
	if err := json.Unmarshal(rec.Body.Bytes(), &infos); err != nil {
		t.Fatalf("parsing torrents info: %v", err)
	}
	if len(infos) != 1 || infos[0].Hash != hashA {
		t.Errorf("expected only the tv torrent, got %+v", infos)
	}
}

func TestAddMagnet(t *testing.T) {
	t.Parallel()

	stub := &stubBittorrent{}
	h := newTestHandler(stub)

	form := url.Values{}
	form.Set("urls", "magnet:?xt=urn:btih:deadbeef")
	form.Set("category", "tv-sonarr")

	rec := doRequest(t, h, http.MethodPost, "/api/v2/torrents/add",
		strings.NewReader(form.Encode()), "application/x-www-form-urlencoded")

	if got := strings.TrimSpace(rec.Body.String()); got != okResponse {
		t.Fatalf("expected %s, got %q (status %d)", okResponse, got, rec.Code)
	}
	stub.mu.Lock()
	defer stub.mu.Unlock()
	if len(stub.magnets) != 1 || stub.magnets[0] != "magnet:?xt=urn:btih:deadbeef" {
		t.Errorf("expected magnet recorded, got %v", stub.magnets)
	}
	for _, cat := range stub.categories {
		if cat != "tv-sonarr" {
			t.Errorf("expected category tv-sonarr, got %q", cat)
		}
	}
}

func TestAddTorrentURL(t *testing.T) {
	t.Parallel()

	stub := &stubBittorrent{}
	h := newTestHandler(stub)

	// Serve some fake .torrent bytes over HTTP.
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("d8:announce42:http://tracker.example.com/announcee"))
	}))
	defer backend.Close()

	form := url.Values{}
	form.Set("urls", backend.URL)

	rec := doRequest(t, h, http.MethodPost, "/api/v2/torrents/add",
		strings.NewReader(form.Encode()), "application/x-www-form-urlencoded")

	if got := strings.TrimSpace(rec.Body.String()); got != okResponse {
		t.Fatalf("expected %s, got %q (status %d)", okResponse, got, rec.Code)
	}
	stub.mu.Lock()
	defer stub.mu.Unlock()
	if len(stub.files) != 1 {
		t.Fatalf("expected one torrent file added, got %d", len(stub.files))
	}
}

func TestAddUploadedTorrent(t *testing.T) {
	t.Parallel()

	stub := &stubBittorrent{}
	h := newTestHandler(stub)

	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	part, err := writer.CreateFormFile("torrents", "show.torrent")
	if err != nil {
		t.Fatalf("creating form file: %v", err)
	}
	_, _ = part.Write([]byte("fake torrent bytes"))
	if err := writer.Close(); err != nil {
		t.Fatalf("closing writer: %v", err)
	}

	rec := doRequest(t, h, http.MethodPost, "/api/v2/torrents/add", &body, writer.FormDataContentType())
	if got := strings.TrimSpace(rec.Body.String()); got != okResponse {
		t.Fatalf("expected %s, got %q (status %d)", okResponse, got, rec.Code)
	}
	stub.mu.Lock()
	defer stub.mu.Unlock()
	if len(stub.files) != 1 || stub.files[0] != "show.torrent" {
		t.Errorf("expected uploaded torrent file recorded, got %v", stub.files)
	}
}

func TestDeleteTorrents(t *testing.T) {
	t.Parallel()

	stub := &stubBittorrent{}
	h := newTestHandler(stub)

	form := url.Values{}
	form.Set("hashes", hashA+",bbbb")
	rec := doRequest(t, h, http.MethodPost, "/api/v2/torrents/delete",
		strings.NewReader(form.Encode()), "application/x-www-form-urlencoded")

	if got := strings.TrimSpace(rec.Body.String()); got != okResponse {
		t.Fatalf("expected %s, got %q", okResponse, got)
	}
	if len(stub.removed) != 2 {
		t.Errorf("expected 2 removals, got %v", stub.removed)
	}
}

func TestAuthRequired(t *testing.T) {
	t.Parallel()

	stub := &stubBittorrent{}
	h := newHandler(handlerSettings{
		savePath:   testSavePath,
		username:   "admin",
		password:   "secret",
		dhtEnabled: true,
		bittorrent: stub,
		logger:     stubLogger{},
	})

	// Without a session, guarded endpoints return 403.
	rec := doRequest(t, h, http.MethodGet, "/api/v2/torrents/info", nil, "")
	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected 403 without login, got %d", rec.Code)
	}

	// Bad credentials -> Fails.
	badForm := url.Values{}
	badForm.Set("username", "admin")
	badForm.Set("password", "wrong")
	rec = doRequest(t, h, http.MethodPost, "/api/v2/auth/login",
		strings.NewReader(badForm.Encode()), "application/x-www-form-urlencoded")
	if got := strings.TrimSpace(rec.Body.String()); got != "Fails." {
		t.Fatalf("expected Fails. for bad login, got %q", got)
	}

	// Good credentials -> Ok. + session cookie.
	goodForm := url.Values{}
	goodForm.Set("username", "admin")
	goodForm.Set("password", "secret")
	rec = doRequest(t, h, http.MethodPost, "/api/v2/auth/login",
		strings.NewReader(goodForm.Encode()), "application/x-www-form-urlencoded")
	if got := strings.TrimSpace(rec.Body.String()); got != okResponse {
		t.Fatalf("expected %s for good login, got %q", okResponse, got)
	}

	var sid *http.Cookie
	for _, c := range rec.Result().Cookies() {
		if c.Name == sessionCookieName {
			sid = c
			break
		}
	}
	if sid == nil {
		t.Fatal("expected SID cookie to be set")
	}

	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/api/v2/torrents/info", nil)
	req.AddCookie(sid)
	infoRec := httptest.NewRecorder()
	h.ServeHTTP(infoRec, req)
	if infoRec.Code != http.StatusOK {
		t.Fatalf("expected 200 with valid session, got %d", infoRec.Code)
	}
}
