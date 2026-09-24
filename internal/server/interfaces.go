package server

import (
	"context"

	"github.com/qdm12/gluetun/internal/configuration/settings"
	"github.com/qdm12/gluetun/internal/models"
)

type VPNLooper interface {
	GetStatus() (status models.LoopStatus)
	ApplyStatus(ctx context.Context, status models.LoopStatus) (
		outcome string, err error)
	GetSettings() (settings settings.VPN)
	SetSettings(ctx context.Context, settings settings.VPN) (outcome string)
}

type DNSLoop interface {
	ApplyStatus(ctx context.Context, status models.LoopStatus) (
		outcome string, err error)
	GetStatus() (status models.LoopStatus)
}

type PortForwardedGetter interface {
	GetPortsForwarded() (ports []uint16)
}

type PublicIPLoop interface {
	GetData() (data models.PublicIP)
}

type Bittorrent interface {
	AddTorrent(magnet string) (infoHash string, err error)
	ListTorrents() (torrents []models.BittorrentTorrent)
	RemoveTorrent(infoHash string) (err error)
}

type Storage interface {
	GetFilterChoices(provider string) models.FilterChoices
}
