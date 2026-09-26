// Package webui implements a small, optional web UI that
// exposes the control server API through a browser interface.
// When enabled it serves a single embedded HTML page and
// reverse-proxies /v1/* requests to the control server,
// injecting the configured API key.
package webui

import (
	"fmt"

	"github.com/qdm12/gluetun/internal/httpserver"
)

// Logger is the logger interface accepted by the web UI server.
type Logger interface {
	Info(msg string)
	Warn(msg string)
	Error(msg string)
}

// Settings configures the web UI server.
type Settings struct {
	// Port is the listening port for the web UI server.
	// It cannot be nil in the internal state.
	Port *uint16
	// ControlServerAddress is the listening address of the
	// control server (for example ":8000") that /v1/* requests
	// are proxied to. It cannot be empty.
	ControlServerAddress string
	// APIKey is the API key injected into proxied requests so
	// the browser does not need to provide it. It can be an
	// empty string if the control server has no API key auth.
	APIKey string
	// Logger is the logger to use. It cannot be nil.
	Logger Logger
}

// New creates the web UI server listening on the configured
// port. Callers should only call New (and run the returned
// server) when the web UI is enabled.
func New(settings Settings) (server *httpserver.Server, err error) {
	proxy, err := newReverseProxy(settings.ControlServerAddress, settings.APIKey)
	if err != nil {
		return nil, fmt.Errorf("creating reverse proxy: %w", err)
	}

	handler := newHandler(proxy)

	httpSettings := httpserver.Settings{
		Address: fmt.Sprintf(":%d", *settings.Port),
		Handler: handler,
		Logger:  settings.Logger,
	}

	server, err = httpserver.New(httpSettings)
	if err != nil {
		return nil, fmt.Errorf("creating web UI server: %w", err)
	}

	return server, nil
}
