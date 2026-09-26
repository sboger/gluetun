// Package qbitapi implements a qBittorrent Web API v2 compatible
// HTTP listener. It has no browser UI; it exists solely so that
// external download managers such as Sonarr and Radarr can be pointed
// at gluetun (host + this port) as a "qBittorrent" download client and
// hand torrents to the embedded BitTorrent client.
//
// Unlike a real qBittorrent, the underlying client only ingests magnet
// links (and, for compatibility, .torrent URLs / uploaded files), so
// the "/api/v2/torrents/add" endpoint accepts those forms.
package qbitapi

import (
	"fmt"

	"github.com/qdm12/gluetun/internal/httpserver"
	"github.com/qdm12/gluetun/internal/models"
)

// Logger is the logger interface accepted by the qBittorrent API server.
type Logger interface {
	Info(msg string)
	Warn(msg string)
	Error(msg string)
}

// Bittorrent is the subset of the embedded BitTorrent client that the
// qBittorrent API needs.
type Bittorrent interface {
	// AddMagnet adds a torrent from a magnet URI, assigning it the
	// given category, and returns its hex encoded info hash.
	AddMagnet(magnet string, category string) (infoHash string, err error)
	// AddTorrentFile adds a torrent from the raw bytes of a .torrent
	// file, assigning it the given category, and returns its hex
	// encoded info hash.
	AddTorrentFile(name string, data []byte, category string) (infoHash string, err error)
	// ListTorrents returns the active torrents of the client.
	ListTorrents() (torrents []models.BittorrentTorrent)
	// RemoveTorrent removes a torrent by its hex encoded info hash.
	RemoveTorrent(infoHash string) (err error)
}

// Settings configures the qBittorrent-compatible API server.
type Settings struct {
	// Port is the listening port for the API server.
	// It cannot be nil in the internal state and defaults to 8080.
	Port *uint16
	// SavePath is the directory torrent data is stored in, reported
	// as the default save path and per-torrent save path.
	// It cannot be empty.
	SavePath string
	// Username and Password enable optional HTTP session
	// authentication. When both are non-empty, every API request
	// (except /auth/login) requires a valid session cookie. When
	// either is empty, no authentication is required.
	Username string
	Password string
	// DHTEnabled reports whether the embedded client uses DHT peer
	// discovery, exposed in the app preferences so Sonarr/Radarr know
	// whether magnet links need explicit trackers.
	DHTEnabled bool
	// Bittorrent is the embedded BitTorrent client. It cannot be nil.
	Bittorrent Bittorrent
	// Logger is the logger to use. It cannot be nil.
	Logger Logger
}

// New creates the qBittorrent-compatible API server listening on the
// configured port. Callers should only call New (and run the returned
// server) when the API is enabled.
func New(settings Settings) (server *httpserver.Server, err error) {
	handler := newHandler(handlerSettings{
		savePath:   settings.SavePath,
		username:   settings.Username,
		password:   settings.Password,
		dhtEnabled: settings.DHTEnabled,
		bittorrent: settings.Bittorrent,
		logger:     settings.Logger,
	})

	httpSettings := httpserver.Settings{
		Address: fmt.Sprintf(":%d", *settings.Port),
		Handler: handler,
		Logger:  settings.Logger,
	}

	server, err = httpserver.New(httpSettings)
	if err != nil {
		return nil, fmt.Errorf("creating qBittorrent API server: %w", err)
	}

	return server, nil
}
