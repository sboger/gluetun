package settings

import (
	"errors"

	"github.com/qdm12/gosettings"
	"github.com/qdm12/gosettings/reader"
	"github.com/qdm12/gotree"
)

var (
	// ErrDownloadDirectoryEmpty is returned when the BitTorrent client
	// is enabled with an empty download directory.
	ErrDownloadDirectoryEmpty = errors.New("download directory is empty")
	// ErrPortZero is returned when the BitTorrent client is configured
	// with a listening port of 0.
	ErrPortZero = errors.New("port cannot be 0")
)

// Bittorrent contains settings to configure the embedded
// BitTorrent client.
type Bittorrent struct {
	// Enabled is true if the embedded BitTorrent client
	// should run. It cannot be nil in the internal state.
	Enabled *bool
	// Port is the listening port for the BitTorrent client.
	// It is nil if not set, in which case the port forwarded
	// by the VPN is used if port forwarding is enabled, or a
	// random free port is used otherwise.
	Port *uint16
	// DownloadDirectory is the directory where downloaded
	// torrent data is stored. It cannot be empty in the
	// internal state when the client is enabled.
	DownloadDirectory string
	// DHTEnabled is true if the Distributed Hash Table peer
	// discovery should be used. It cannot be nil in the
	// internal state.
	DHTEnabled *bool
	// UploadRate is the maximum upload speed in bytes per
	// second. It is nil if unlimited, in which case there is
	// no upload bandwidth limit.
	UploadRate *uint64
	// DownloadRate is the maximum download speed in bytes per
	// second. It is nil if unlimited, in which case there is
	// no download bandwidth limit.
	DownloadRate *uint64
}

func (b Bittorrent) validate() (err error) {
	if !*b.Enabled {
		return nil
	}

	if b.DownloadDirectory == "" {
		return ErrDownloadDirectoryEmpty
	}

	if b.Port != nil && *b.Port == 0 {
		return ErrPortZero
	}

	return nil
}

func (b *Bittorrent) copy() (copied Bittorrent) {
	return Bittorrent{
		Enabled:           gosettings.CopyPointer(b.Enabled),
		Port:              gosettings.CopyPointer(b.Port),
		DownloadDirectory: b.DownloadDirectory,
		DHTEnabled:        gosettings.CopyPointer(b.DHTEnabled),
		UploadRate:        gosettings.CopyPointer(b.UploadRate),
		DownloadRate:      gosettings.CopyPointer(b.DownloadRate),
	}
}

// overrideWith overrides fields of the receiver
// settings object with any field set in the other
// settings.
func (b *Bittorrent) overrideWith(other Bittorrent) {
	b.Enabled = gosettings.OverrideWithPointer(b.Enabled, other.Enabled)
	b.Port = gosettings.OverrideWithPointer(b.Port, other.Port)
	b.DownloadDirectory = gosettings.OverrideWithComparable(b.DownloadDirectory, other.DownloadDirectory)
	b.DHTEnabled = gosettings.OverrideWithPointer(b.DHTEnabled, other.DHTEnabled)
	b.UploadRate = gosettings.OverrideWithPointer(b.UploadRate, other.UploadRate)
	b.DownloadRate = gosettings.OverrideWithPointer(b.DownloadRate, other.DownloadRate)
}

func (b *Bittorrent) setDefaults() {
	b.Enabled = gosettings.DefaultPointer(b.Enabled, false)
	const defaultDownloadDirectory = "/downloads"
	b.DownloadDirectory = gosettings.DefaultComparable(b.DownloadDirectory, defaultDownloadDirectory)
	b.DHTEnabled = gosettings.DefaultPointer(b.DHTEnabled, true)
}

func (b Bittorrent) String() string {
	return b.toLinesNode().String()
}

func (b Bittorrent) toLinesNode() (node *gotree.Node) {
	node = gotree.New("BitTorrent client settings:")
	node.Appendf("Enabled: %s", gosettings.BoolToYesNo(b.Enabled))
	if !*b.Enabled {
		return node
	}

	switch b.Port {
	case nil:
		node.Appendf("Port: auto (VPN forwarded port if enabled)")
	default:
		node.Appendf("Port: %d", *b.Port)
	}

	node.Appendf("Download directory: %s", b.DownloadDirectory)
	node.Appendf("DHT: %s", gosettings.BoolToYesNo(b.DHTEnabled))
	switch b.UploadRate {
	case nil:
		node.Appendf("Upload rate: unlimited")
	default:
		node.Appendf("Upload rate: %d bytes/s", *b.UploadRate)
	}
	switch b.DownloadRate {
	case nil:
		node.Appendf("Download rate: unlimited")
	default:
		node.Appendf("Download rate: %d bytes/s", *b.DownloadRate)
	}

	return node
}

func (b *Bittorrent) read(r *reader.Reader) (err error) {
	b.Enabled, err = r.BoolPtr("BITTORRENT_CLIENT")
	if err != nil {
		return err
	}

	b.Port, err = r.Uint16Ptr("BITTORRENT_PORT")
	if err != nil {
		return err
	}

	b.DownloadDirectory = r.String("BITTORRENT_DOWNLOAD_DIRECTORY")

	b.DHTEnabled, err = r.BoolPtr("BITTORRENT_DHT")
	if err != nil {
		return err
	}

	b.UploadRate, err = readByteRate(r, "BITTORRENT_UPLOAD_RATE")
	if err != nil {
		return err
	}

	b.DownloadRate, err = readByteRate(r, "BITTORRENT_DOWNLOAD_RATE")
	if err != nil {
		return err
	}

	return nil
}

func readByteRate(r *reader.Reader, key string) (rate *uint64, err error) {
	uintValue, err := r.UintPtr(key)
	if err != nil {
		return nil, err
	}
	if uintValue != nil {
		uint64Value := uint64(*uintValue)
		rate = &uint64Value
	}
	return rate, nil
}
