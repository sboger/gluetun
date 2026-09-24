package bittorrent

import (
	"fmt"

	"github.com/anacrolix/torrent"
	"github.com/anacrolix/torrent/metainfo"
	"github.com/qdm12/gluetun/internal/models"
	"golang.org/x/time/rate"
)

// clientConfig is the runtime configuration of the embedded
// BitTorrent client.
type clientConfig struct {
	// Port is the listening port, 0 selects a random free port.
	Port uint16
	// DownloadDirectory is where torrent data is stored.
	DownloadDirectory string
	// DHTEnabled enables the Distributed Hash Table.
	DHTEnabled bool
	// UploadRate is the upload bandwidth limit in bytes per
	// second, nil for unlimited.
	UploadRate *uint64
	// DownloadRate is the download bandwidth limit in bytes per
	// second, nil for unlimited.
	DownloadRate *uint64
}

// client wraps a github.com/anacrolix/torrent client.
type client struct {
	client       *torrent.Client
	downloadDir  string
	uploadRate   *uint64
	downloadRate *uint64
}

func newClient(config clientConfig) (c *client, err error) {
	// Start from the full default configuration so that anacrolix's
	// default callbacks (DHT starting nodes, metainfo sources merger,
	// etc.) are populated, then override only the fields we control.
	// Passing a bare *ClientConfig to NewClient skips those defaults and
	// panics when DHT is enabled (nil DhtStartingNodes).
	torrentConfig := torrent.NewDefaultClientConfig()
	torrentConfig.DataDir = config.DownloadDirectory
	torrentConfig.ListenPort = int(config.Port)
	// Gluetun's firewall already handles port forwarding and only
	// allows explicitly opened ports through the tunnel, so we disable
	// anacrolix's default UPnP port forwarding.
	torrentConfig.NoDefaultPortForwarding = true
	// NoDHT is promoted from the embedded ClientDhtConfig, so it cannot
	// be set in a struct literal at go 1.26 language level.
	torrentConfig.NoDHT = !config.DHTEnabled

	// anacrolix/torrent v1.61's ClientConfig.setRateLimiterBursts calls
	// UploadRateLimiter.Burst() unconditionally, so both limiters must
	// never be nil. Use an unlimited limiter when no rate is configured.
	const limiterBurst = 1 << 20 // 1 MiB, large enough for a full chunk
	torrentConfig.UploadRateLimiter = newRateLimiter(config.UploadRate, limiterBurst)
	torrentConfig.DownloadRateLimiter = newRateLimiter(config.DownloadRate, limiterBurst)

	torrentClient, err := torrent.NewClient(torrentConfig)
	if err != nil {
		return nil, fmt.Errorf("creating BitTorrent client: %w", err)
	}

	return &client{
		client:       torrentClient,
		downloadDir:  config.DownloadDirectory,
		uploadRate:   config.UploadRate,
		downloadRate: config.DownloadRate,
	}, nil
}

// newRateLimiter returns a rate limiter bounded by the given byte rate,
// or an unlimited limiter when rate is nil or zero.
func newRateLimiter(ratePerSecond *uint64, burst int) *rate.Limiter {
	if ratePerSecond != nil && *ratePerSecond > 0 {
		return rate.NewLimiter(rate.Limit(*ratePerSecond), burst)
	}
	return rate.NewLimiter(rate.Inf, burst)
}

func (c *client) close() {
	c.client.Close()
}

// addMagnet adds a torrent from a magnet URI and starts downloading it
// automatically once its metadata is obtained. It returns the torrent
// info hash, hex encoded.
func (c *client) addMagnet(magnet string) (infoHash string, err error) {
	metaInfo, err := metainfo.ParseMagnetUri(magnet)
	if err != nil {
		return "", fmt.Errorf("parsing magnet URI: %w", err)
	}

	infoHash = metaInfo.InfoHash.HexString()
	if _, exists := c.client.Torrent(metaInfo.InfoHash); exists {
		return infoHash, fmt.Errorf("torrent %s already exists", infoHash)
	}

	addedTorrent, err := c.client.AddMagnet(magnet)
	if err != nil {
		return "", fmt.Errorf("adding magnet torrent: %w", err)
	}

	c.drive(addedTorrent)
	return infoHash, nil
}

// drive downloads the whole torrent once its metadata is available,
// or stops early if the torrent is closed.
func (c *client) drive(addedTorrent *torrent.Torrent) {
	select {
	case <-addedTorrent.GotInfo():
		addedTorrent.DownloadAll()
	case <-addedTorrent.Closed():
	}
}

func (c *client) listTorrents() (list []models.BittorrentTorrent) {
	torrents := c.client.Torrents()
	list = make([]models.BittorrentTorrent, 0, len(torrents))
	for _, torrent := range torrents {
		list = append(list, torrentInfo(torrent))
	}
	return list
}

func torrentInfo(t *torrent.Torrent) (info models.BittorrentTorrent) {
	info = models.BittorrentTorrent{
		InfoHash: t.InfoHash().HexString(),
		Name:     t.Name(),
	}

	metainfo := t.Info()
	if metainfo == nil {
		info.State = "fetching metadata"
		return info
	}

	const percentScale = 100
	info.Length = t.Length()
	info.BytesCompleted = t.BytesCompleted()
	if info.Length > 0 {
		info.PercentComplete = int(float64(info.BytesCompleted) * percentScale / float64(info.Length))
		if info.PercentComplete > percentScale {
			info.PercentComplete = percentScale
		}
	}
	info.Seeding = t.Seeding()
	switch {
	case info.Seeding:
		info.State = "seeding"
	case info.BytesCompleted < info.Length:
		info.State = "downloading"
	default:
		info.State = "idle"
	}
	return info
}

func (c *client) removeTorrent(infoHashHex string) (err error) {
	var infoHash metainfo.Hash
	if err := infoHash.FromHexString(infoHashHex); err != nil {
		return fmt.Errorf("parsing info hash %q: %w", infoHashHex, err)
	}

	torrent, exists := c.client.Torrent(infoHash)
	if !exists {
		return fmt.Errorf("torrent %s not found", infoHashHex)
	}

	torrent.Drop()
	return nil
}
