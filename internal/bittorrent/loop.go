package bittorrent

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/qdm12/gluetun/internal/models"
)

const portRetryInterval = 5 * time.Second

// ErrClientNotRunning is returned when an operation is attempted
// while the BitTorrent client is not running.
var ErrClientNotRunning = errors.New("BitTorrent client is not running")

// Loop runs the embedded BitTorrent client and exposes it to the
// control server API.
type Loop struct {
	// settings holds the BitTorrent client configuration.
	settings Settings
	// portForwarder provides the VPN forwarded ports.
	portForwarder PortForwarder
	logger        Logger

	// lock guards client.
	lock   sync.RWMutex
	client *client
}

// NewLoop creates a BitTorrent loop. settings.UseForwardedPort is true
// when the VPN provider has port forwarding enabled, so the client's
// listening port automatically follows the forwarded port unless a
// manual port is set.
func NewLoop(settings Settings, portForwarder PortForwarder, logger Logger) *Loop {
	return &Loop{
		settings:      settings,
		portForwarder: portForwarder,
		logger:        logger,
	}
}

// Run runs the BitTorrent client until ctx is done. It blocks until a
// listening port is available when relying on VPN port forwarding.
func (l *Loop) Run(ctx context.Context, done chan<- struct{}) {
	defer close(done)

	if !*l.settings.Enabled {
		return
	}

	l.logger.Info("starting BitTorrent client")

	port, err := l.resolvePort(ctx)
	if err != nil {
		// ctx was canceled before a port could be resolved
		return
	}

	bittorrentClient, err := newClient(clientConfig{
		Port:              port,
		DownloadDirectory: l.settings.DownloadDirectory,
		DHTEnabled:        *l.settings.DHTEnabled,
		UploadRate:        l.settings.UploadRate,
		DownloadRate:      l.settings.DownloadRate,
	})
	if err != nil {
		l.logger.Error("cannot start BitTorrent client: " + err.Error())
		return
	}

	l.lock.Lock()
	l.client = bittorrentClient
	l.lock.Unlock()

	defer func() {
		l.lock.Lock()
		runningClient := l.client
		l.client = nil
		l.lock.Unlock()
		if runningClient != nil {
			runningClient.close()
		}
	}()

	l.logger.Info(fmt.Sprintf("BitTorrent client listening on port %d", port))

	<-ctx.Done()
	l.logger.Info("stopping BitTorrent client")
}

// resolvePort returns the listening port for the BitTorrent client,
// using the manual port if set, the VPN forwarded port otherwise (when
// port forwarding is enabled), or 0 for a random free port.
func (l *Loop) resolvePort(ctx context.Context) (port uint16, err error) {
	if l.settings.Port != nil {
		return *l.settings.Port, nil
	}

	if !l.settings.UseForwardedPort {
		return 0, nil
	}

	timer := time.NewTimer(portRetryInterval)
	defer timer.Stop()
	for {
		ports := l.portForwarder.GetPortsForwarded()
		if len(ports) > 0 {
			return ports[0], nil
		}
		l.logger.Info("waiting for a forwarded port to listen on")
		timer.Reset(portRetryInterval)
		select {
		case <-timer.C:
		case <-ctx.Done():
			return 0, ctx.Err()
		}
	}
}

// AddTorrent adds a torrent from a magnet URI and returns its info
// hash.
func (l *Loop) AddTorrent(magnet string) (infoHash string, err error) {
	l.lock.RLock()
	runningClient := l.client
	l.lock.RUnlock()
	if runningClient == nil {
		return "", ErrClientNotRunning
	}
	return runningClient.addMagnet(magnet)
}

// ListTorrents returns the active torrents of the BitTorrent client.
func (l *Loop) ListTorrents() (torrents []models.BittorrentTorrent) {
	l.lock.RLock()
	runningClient := l.client
	l.lock.RUnlock()
	if runningClient == nil {
		return nil
	}
	return runningClient.listTorrents()
}

// RemoveTorrent removes a torrent by its hex encoded info hash.
func (l *Loop) RemoveTorrent(infoHash string) (err error) {
	l.lock.RLock()
	runningClient := l.client
	l.lock.RUnlock()
	if runningClient == nil {
		return ErrClientNotRunning
	}
	return runningClient.removeTorrent(infoHash)
}

func (l *Loop) String() string {
	return "BitTorrent client"
}
