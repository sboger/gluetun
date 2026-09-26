package settings

import (
	"errors"
	"fmt"
	"os"

	"github.com/qdm12/gosettings"
	"github.com/qdm12/gosettings/reader"
	"github.com/qdm12/gotree"
)

const defaultQbittorrentAPIPort = 8080

// ErrQbittorrentAPIPortZero is returned when the qBittorrent API is
// enabled with a listening port of 0.
var ErrQbittorrentAPIPortZero = errors.New("qBittorrent API port cannot be 0")

// Qbittorrent contains settings to configure the optional
// qBittorrent-compatible API server. It has no browser UI; it exists
// so Sonarr/Radarr can treat gluetun as a qBittorrent download client
// and hand magnet links / torrent URLs to the embedded BitTorrent
// client.
type Qbittorrent struct {
	// Enabled is true if the qBittorrent-compatible API server
	// should run. It cannot be nil in the internal state.
	Enabled *bool
	// Port is the listening port for the qBittorrent-compatible API
	// server. It cannot be nil in the internal state and defaults to
	// 8080.
	Port *uint16
	// Username optionally enables session authentication. When both
	// Username and Password are non-empty, requests must authenticate
	// via /api/v2/auth/login. When either is empty, no authentication
	// is required.
	Username string
	// Password is the password paired with Username.
	Password string
}

func (q *Qbittorrent) validate() (err error) {
	if !*q.Enabled {
		return nil
	}

	if *q.Port == 0 {
		return ErrQbittorrentAPIPortZero
	}

	uid := os.Getuid()
	const maxPrivilegedPort = 1023
	if uid != 0 && *q.Port <= maxPrivilegedPort {
		return fmt.Errorf("%w: %d when running with user ID %d",
			ErrControlServerPrivilegedPort, *q.Port, uid)
	}

	return nil
}

func (q *Qbittorrent) copy() (copied Qbittorrent) {
	return Qbittorrent{
		Enabled:  gosettings.CopyPointer(q.Enabled),
		Port:     gosettings.CopyPointer(q.Port),
		Username: q.Username,
		Password: q.Password,
	}
}

func (q *Qbittorrent) overrideWith(other Qbittorrent) {
	q.Enabled = gosettings.OverrideWithPointer(q.Enabled, other.Enabled)
	q.Port = gosettings.OverrideWithPointer(q.Port, other.Port)
	q.Username = gosettings.OverrideWithComparable(q.Username, other.Username)
	q.Password = gosettings.OverrideWithComparable(q.Password, other.Password)
}

func (q *Qbittorrent) setDefaults() {
	q.Enabled = gosettings.DefaultPointer(q.Enabled, false)
	q.Port = gosettings.DefaultPointer(q.Port, defaultQbittorrentAPIPort)
}

func (q Qbittorrent) String() string {
	return q.toLinesNode().String()
}

func (q Qbittorrent) toLinesNode() (node *gotree.Node) {
	node = gotree.New("qBittorrent-compatible API settings:")
	node.Appendf("Enabled: %s", gosettings.BoolToYesNo(q.Enabled))
	if !*q.Enabled {
		return node
	}
	node.Appendf("Port: %d", *q.Port)
	authEnabled := q.Username != "" && q.Password != ""
	node.Appendf("Authentication: %s", gosettings.BoolToYesNo(&authEnabled))
	return node
}

func (q *Qbittorrent) read(r *reader.Reader) (err error) {
	q.Enabled, err = r.BoolPtr("QBITTORRENT_API")
	if err != nil {
		return err
	}

	q.Port, err = r.Uint16Ptr("QBITTORRENT_API_PORT")
	if err != nil {
		return err
	}

	q.Username = r.String("QBITTORRENT_API_USERNAME")
	q.Password = r.String("QBITTORRENT_API_PASSWORD")

	return nil
}
