package bittorrent

import "github.com/qdm12/gluetun/internal/configuration/settings"

// PortForwarder provides the ports forwarded by the VPN,
// used to automatically set the BitTorrent listening port.
type PortForwarder interface {
	GetPortsForwarded() (ports []uint16)
}

// Logger is a subset of gluetun's logging interface.
type Logger interface {
	Info(s string)
	Error(s string)
}

// Settings is the configuration consumed by the BitTorrent loop.
type Settings struct {
	settings.Bittorrent
	// UseForwardedPort is true if the BitTorrent client should
	// listen on the port forwarded by the VPN when no manual
	// port is set.
	UseForwardedPort bool
}
