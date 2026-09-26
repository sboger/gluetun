package qbitapi

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/qdm12/gluetun/internal/models"
)

const (
	qbitAppVersion     = "4.3.9"
	qbitWebAPIVersion  = "2.8.1"
	completedETA       = 8640000 // qBittorrent's sentinel ETA for completed torrents
	completePercent    = 100
	maxTorrentFileSize = 100 << 20
	formMaxMemory      = 32 << 20
	sessionIDSize      = 16
	httpClientTimeout  = 30 * time.Second
	fetchTimeout       = 60 * time.Second

	sessionCookieName = "SID"
)

var (
	// ErrTorrentFetchFailed is returned when a torrent URL could not
	// be fetched or returned a non-success status.
	ErrTorrentFetchFailed = errors.New("fetching torrent URL failed")
	// ErrTorrentEmpty is returned when a fetched torrent URL had an
	// empty body.
	ErrTorrentEmpty = errors.New("fetched empty .torrent file")
)

// handlerSettings are the resolved configuration of the API handler.
type handlerSettings struct {
	savePath   string
	username   string
	password   string
	bittorrent Bittorrent
	dhtEnabled bool
	logger     Logger
}

// handler implements the qBittorrent Web API v2 compatible layer on
// top of the embedded BitTorrent client.
type handler struct {
	savePath   string
	username   string
	password   string
	bittorrent Bittorrent
	dhtEnabled bool
	logger     Logger

	authRequired bool
	sid          string

	httpClient *http.Client
	tracker    *speedTracker
}

func newHandler(settings handlerSettings) *handler {
	sid := make([]byte, sessionIDSize)
	_, _ = rand.Read(sid)

	return &handler{
		savePath:     settings.savePath,
		username:     settings.username,
		password:     settings.password,
		bittorrent:   settings.bittorrent,
		dhtEnabled:   settings.dhtEnabled,
		logger:       settings.logger,
		authRequired: settings.username != "" && settings.password != "",
		sid:          hex.EncodeToString(sid),
		httpClient:   &http.Client{Timeout: httpClientTimeout},
		tracker:      newSpeedTracker(),
	}
}

func (h *handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// No browser UI here: every valid path lives under /api/v2/.
	if !strings.HasPrefix(r.URL.Path, "/api/v2/") {
		http.NotFound(w, r)
		return
	}

	path := strings.TrimPrefix(r.URL.Path, "/api/v2/")

	// Authentication: allow login without a session, require a valid
	// session cookie for everything else when auth is enabled.
	if r.Method != http.MethodGet && path == "auth/login" {
		h.login(w, r)
		return
	}
	if path == "auth/logout" {
		h.logout(w)
		return
	}
	if h.authRequired && !h.sessionValid(r) {
		http.Error(w, http.StatusText(http.StatusForbidden), http.StatusForbidden)
		return
	}

	switch path {
	case "app/version":
		writePlain(w, qbitAppVersion)
	case "app/webapiVersion":
		writePlain(w, qbitWebAPIVersion)
	case "app/buildInfo":
		writeJSON(w, qbitBuildInfo())
	case "app/preferences":
		writeJSON(w, h.qbitPreferences())
	case "app/defaultSavePath":
		writePlain(w, h.savePath)
	case "torrents/info":
		h.listTorrents(w, r)
	case "torrents/add":
		h.addTorrent(w, r)
	case "torrents/delete":
		h.deleteTorrents(w, r)
	case "torrents/pause", "torrents/resume", "torrents/reannounce",
		"torrents/setCategory", "torrents/createCategory", "torrents/editCategory",
		"torrents/setShareLimits", "torrents/topPrio", "torrents/bottomPrio":
		writePlain(w, "Ok.")
	case "torrents/properties":
		h.torrentProperties(w, r)
	case "torrents/files":
		h.torrentFiles(w, r)
	case "torrents/categories":
		// The embedded client has no named categories; report an
		// empty set which Sonarr/Radarr treat as all default.
		writeJSON(w, map[string]qbitCategory{})
	default:
		http.NotFound(w, r)
	}
}

func (h *handler) login(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseMultipartForm(0); err != nil && !errors.Is(err, http.ErrNotMultipart) {
		writePlain(w, "Fails.")
		return
	}

	if h.authRequired &&
		(r.FormValue("username") != h.username || r.FormValue("password") != h.password) {
		writePlain(w, "Fails.")
		return
	}

	//nolint:gosec // Secure would break cookie round-trip over the plain-HTTP LAN.
	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookieName,
		Value:    h.sid,
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
	})
	writePlain(w, "Ok.")
}

func (h *handler) logout(w http.ResponseWriter) {
	//nolint:gosec // See login: Secure is inappropriate on plain HTTP.
	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookieName,
		Value:    "",
		MaxAge:   -1,
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
	})
	writePlain(w, "Ok.")
}

func (h *handler) sessionValid(r *http.Request) bool {
	cookie, err := r.Cookie(sessionCookieName)
	return err == nil && cookie.Value == h.sid
}

// qbitPreferences maps the API's preferences to what Sonarr/Radarr
// read when building their download client item and seed limits.
func (h *handler) qbitPreferences() qbitPreferences {
	return qbitPreferences{
		DhtEnabled:                    h.dhtEnabled,
		SavePath:                      h.savePath,
		MaxRatioEnabled:               false,
		MaxRatio:                      -1,
		MaxSeedingTimeEnabled:         false,
		MaxSeedingTime:                -1,
		MaxInactiveSeedingTimeEnabled: false,
		MaxInactiveSeedingTime:        -1,
	}
}

func (h *handler) listTorrents(w http.ResponseWriter, r *http.Request) {
	categoryFilter := r.URL.Query().Get("category")

	torrents := h.bittorrent.ListTorrents()
	output := make([]qbitTorrentInfo, 0, len(torrents))
	now := time.Now()

	for _, torrent := range torrents {
		if categoryFilter != "" && torrent.Category != categoryFilter {
			continue
		}
		downSpeed, upSpeed := h.tracker.rates(torrent.InfoHash, torrent.Downloaded, torrent.Uploaded, now)
		output = append(output, h.toTorrentInfo(torrent, downSpeed, upSpeed))
	}

	writeJSON(w, output)
}

func (h *handler) toTorrentInfo(t models.BittorrentTorrent, downSpeed, upSpeed int64) (info qbitTorrentInfo) {
	progress := 0.0
	if t.Length > 0 {
		progress = float64(t.BytesCompleted) / float64(t.Length)
		if progress > 1 {
			progress = 1
		}
	}

	complete := t.PercentComplete >= completePercent
	state := "downloading"
	eta := int64(-1)
	switch {
	case t.State == "fetching metadata":
		state = "metaDL"
	case complete:
		state = "uploading"
		eta = completedETA
	case downSpeed > 0 && t.Length > t.BytesCompleted:
		eta = (t.Length - t.BytesCompleted) / downSpeed
	}

	ratio := 0.0
	if t.Downloaded > 0 {
		ratio = float64(t.Uploaded) / float64(t.Downloaded)
	}

	return qbitTorrentInfo{
		Hash:                t.InfoHash,
		Name:                t.Name,
		Size:                t.Length,
		Progress:            progress,
		Downloaded:          t.Downloaded,
		Uploaded:            t.Uploaded,
		AmountLeft:          t.Length - t.BytesCompleted,
		ETA:                 eta,
		State:               state,
		DownloadSpeed:       downSpeed,
		UploadSpeed:         upSpeed,
		Ratio:               ratio,
		Category:            t.Category,
		Label:               t.Category,
		SavePath:            t.SavePath,
		ContentPath:         t.ContentPath,
		NumSeeds:            t.NumSeeds,
		NumLeechs:           t.NumLeechs,
		TimeAdded:           t.TimeAdded,
		SeedingTime:         0,
		LastActivity:        t.TimeAdded,
		RatioLimit:          -2,
		SeedingTimeLimit:    -2,
		InactiveSeedingTime: -2,
		TotalSeeds:          t.NumSeeds,
		TotalLeechs:         t.NumLeechs,
	}
}

func (h *handler) addTorrent(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseMultipartForm(formMaxMemory); err != nil && !errors.Is(err, http.ErrNotMultipart) {
		h.logger.Warn("qBittorrent add failed parsing form: " + err.Error())
		writePlain(w, "Fails.")
		return
	}

	category := r.FormValue("category")
	added := 0

	for _, entry := range strings.Split(r.FormValue("urls"), ",") {
		entry = strings.TrimSpace(entry)
		if entry == "" {
			continue
		}
		if err := h.addSource(entry, category); err != nil {
			h.logger.Warn("qBittorrent add failed for " + entry + ": " + err.Error())
			continue
		}
		added++
	}

	if r.MultipartForm != nil {
		for _, fileHeader := range r.MultipartForm.File["torrents"] {
			data, err := readUpload(fileHeader)
			if err != nil {
				h.logger.Warn("qBittorrent add failed reading upload: " + err.Error())
				continue
			}
			if _, err := h.bittorrent.AddTorrentFile(fileHeader.Filename, data, category); err != nil {
				h.logger.Warn("qBittorrent add failed for uploaded torrent: " + err.Error())
				continue
			}
			added++
		}
	}

	if added > 0 {
		writePlain(w, "Ok.")
		return
	}
	writePlain(w, "Fails.")
}

// addSource adds a torrent from either a magnet link or a URL to a
// .torrent file, matching what Sonarr/Radarr send in the "urls" field.
func (h *handler) addSource(source string, category string) (err error) {
	if strings.HasPrefix(source, "magnet:") {
		_, err := h.bittorrent.AddMagnet(source, category)
		return err
	}

	// Otherwise treat the source as a URL to a .torrent file and
	// fetch it, for compatibility with clients that point at a real
	// torrent URL instead of a magnet.
	data, err := h.fetch(source)
	if err != nil {
		return err
	}
	_, err = h.bittorrent.AddTorrentFile(source, data, category)
	return err
}

func (h *handler) fetch(urlStr string) (data []byte, err error) {
	ctx, cancel := context.WithTimeout(context.Background(), fetchTimeout)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, urlStr, nil)
	if err != nil {
		return nil, fmt.Errorf("building torrent fetch request: %w", err)
	}

	resp, err := h.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetching torrent URL: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return nil, fmt.Errorf("%w: %s returned status %d",
			ErrTorrentFetchFailed, urlStr, resp.StatusCode)
	}

	data, err = io.ReadAll(io.LimitReader(resp.Body, maxTorrentFileSize))
	if err != nil {
		return nil, fmt.Errorf("reading torrent URL body: %w", err)
	}
	if len(data) == 0 {
		return nil, fmt.Errorf("%w: %s", ErrTorrentEmpty, urlStr)
	}
	return data, nil
}

// readUpload returns the contents of an uploaded .torrent file.
func readUpload(fileHeader *multipart.FileHeader) (data []byte, err error) {
	file, err := fileHeader.Open()
	if err != nil {
		return nil, err
	}
	defer file.Close()
	return io.ReadAll(io.LimitReader(file, maxTorrentFileSize))
}

func (h *handler) deleteTorrents(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseMultipartForm(0); err != nil && !errors.Is(err, http.ErrNotMultipart) {
		writePlain(w, "Fails.")
		return
	}

	for _, hash := range strings.Split(r.FormValue("hashes"), ",") {
		hash = strings.TrimSpace(hash)
		if hash == "" {
			continue
		}
		if err := h.bittorrent.RemoveTorrent(hash); err != nil {
			h.logger.Warn("qBittorrent delete failed for " + hash + ": " + err.Error())
		}
	}

	writePlain(w, "Ok.")
}

func (h *handler) torrentProperties(w http.ResponseWriter, r *http.Request) {
	hash := r.URL.Query().Get("hash")
	torrent := h.findTorrent(strings.ToLower(hash))
	if torrent == nil {
		http.NotFound(w, r)
		return
	}

	size := torrent.Length
	progress := 0.0
	if size > 0 {
		progress = float64(torrent.BytesCompleted) / float64(size)
		if progress > 1 {
			progress = 1
		}
	}
	ratio := 0.0
	if torrent.Downloaded > 0 {
		ratio = float64(torrent.Uploaded) / float64(torrent.Downloaded)
	}
	completedOn := int64(0)
	if torrent.PercentComplete >= completePercent {
		completedOn = torrent.TimeAdded
	}

	properties := qbitTorrentProperties{
		SavePath:        torrent.SavePath,
		CreationDate:    torrent.TimeAdded,
		Comment:         "",
		TotalWasted:     0,
		TotalUploaded:   torrent.Uploaded,
		TotalDownloaded: torrent.Downloaded,
		UploadRatio:     ratio,
		DownloadPath:    torrent.SavePath,
		AddedOn:         torrent.TimeAdded,
		CompletedOn:     completedOn,
		FileCount:       len(torrent.Files),
		Hash:            torrent.InfoHash,
		Name:            torrent.Name,
		Size:            size,
		Progress:        progress,
		PieceSize:       0,
		SeedingTime:     0,
	}

	writeJSON(w, properties)
}

func (h *handler) torrentFiles(w http.ResponseWriter, r *http.Request) {
	hash := r.URL.Query().Get("hash")
	torrent := h.findTorrent(strings.ToLower(hash))
	if torrent == nil {
		http.NotFound(w, r)
		return
	}

	files := make([]qbitTorrentFile, 0, len(torrent.Files))
	for _, file := range torrent.Files {
		size := file.Length
		progress := 0.0
		if size > 0 {
			progress = float64(file.BytesCompleted) / float64(size)
			if progress > 1 {
				progress = 1
			}
		}
		priority := 1 // normal
		if progress >= 1 {
			priority = 6 // normal + do not download is 0; completed files sit at unknown
		}
		isSeed := 0
		if progress >= 1 {
			isSeed = 1
		}
		files = append(files, qbitTorrentFile{
			Index:    file.Index,
			Name:     file.Name,
			Size:     size,
			Progress: progress,
			Priority: priority,
			IsSeed:   isSeed,
		})
	}

	writeJSON(w, files)
}

func (h *handler) findTorrent(hash string) (found *models.BittorrentTorrent) {
	if hash == "" {
		return nil
	}
	torrents := h.bittorrent.ListTorrents()
	for i := range torrents {
		if strings.EqualFold(torrents[i].InfoHash, hash) {
			return &torrents[i]
		}
	}
	return nil
}

func writeJSON[T any](w http.ResponseWriter, value T) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	encoder := json.NewEncoder(w)
	if err := encoder.Encode(value); err != nil {
		// Headers have already been sent, so there is nothing to do
		// but swallow the error.
		_ = err
	}
}

func writePlain(w http.ResponseWriter, text string) {
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	_, _ = io.WriteString(w, text+"\n")
}

// speedTracker computes per-second transfer rates from cumulative byte
// counters sampled across successive /torrents/info calls.
type speedTracker struct {
	mu   sync.Mutex
	last map[string]speedSample
}

type speedSample struct {
	downloaded int64
	uploaded   int64
	at         time.Time
}

func newSpeedTracker() *speedTracker {
	return &speedTracker{last: make(map[string]speedSample)}
}

func (s *speedTracker) rates(infoHash string, downloaded, uploaded int64, now time.Time) (down, up int64) {
	s.mu.Lock()
	defer s.mu.Unlock()

	previous, exists := s.last[infoHash]
	s.last[infoHash] = speedSample{downloaded: downloaded, uploaded: uploaded, at: now}
	if !exists {
		return 0, 0
	}

	seconds := now.Sub(previous.at).Seconds()
	if seconds <= 0 {
		return 0, 0
	}
	if downloaded >= previous.downloaded {
		down = int64(float64(downloaded-previous.downloaded) / seconds)
	}
	if uploaded >= previous.uploaded {
		up = int64(float64(uploaded-previous.uploaded) / seconds)
	}
	return down, up
}
