package bittorrent

import (
	"bytes"
	"errors"
	"fmt"
	"path/filepath"
	"sync"
	"time"

	"github.com/anacrolix/torrent"
	"github.com/anacrolix/torrent/metainfo"
	"github.com/qdm12/gluetun/internal/models"
	"golang.org/x/time/rate"
)

var (
	// ErrTorrentExists is returned when adding a torrent whose
	// info hash is already managed by the client.
	ErrTorrentExists = errors.New("torrent already exists")
	// ErrTorrentNotFound is returned when removing a torrent whose
	// info hash is not managed by the client.
	ErrTorrentNotFound = errors.New("torrent not found")
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

// torrentMeta is per-torrent metadata tracked by the client,
// since the underlying anacrolix torrent has no notion of a
// category or a creation timestamp.
type torrentMeta struct {
	category string
	addedAt  int64 // Unix seconds
}

// client wraps a github.com/anacrolix/torrent client.
type client struct {
	client       *torrent.Client
	downloadDir  string
	uploadRate   *uint64
	downloadRate *uint64

	// metaLock guards meta, which is keyed by hex info hash.
	metaLock sync.RWMutex
	meta     map[string]torrentMeta
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
		meta:         make(map[string]torrentMeta),
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

func (c *client) setMeta(infoHashHex string, meta torrentMeta) {
	c.metaLock.Lock()
	defer c.metaLock.Unlock()
	c.meta[infoHashHex] = meta
}

func (c *client) getMeta(infoHashHex string) (meta torrentMeta) {
	c.metaLock.RLock()
	defer c.metaLock.RUnlock()
	meta, _ = c.meta[infoHashHex]
	return meta
}

func (c *client) deleteMeta(infoHashHex string) {
	c.metaLock.Lock()
	defer c.metaLock.Unlock()
	delete(c.meta, infoHashHex)
}

// addMagnet adds a torrent from a magnet URI and starts downloading it
// automatically once its metadata is obtained. It returns the torrent
// info hash, hex encoded. The category is assigned to the torrent and
// exposed through the APIs.
func (c *client) addMagnet(magnet string, category string) (infoHash string, err error) {
	metaInfo, err := metainfo.ParseMagnetUri(magnet)
	if err != nil {
		return "", fmt.Errorf("parsing magnet URI: %w", err)
	}

	return c.addTorrent(metaInfo.InfoHash, category, func() (*torrent.Torrent, error) {
		return c.client.AddMagnet(magnet)
	})
}

// addTorrentFile adds a torrent from the raw bytes of a .torrent file
// and starts downloading it automatically. It returns the torrent info
// hash, hex encoded.
func (c *client) addTorrentFile(data []byte, category string) (infoHash string, err error) {
	metaInfo, err := metainfo.Load(bytes.NewReader(data))
	if err != nil {
		return "", fmt.Errorf("parsing .torrent file: %w", err)
	}

	return c.addTorrent(metaInfo.HashInfoBytes(), category, func() (*torrent.Torrent, error) {
		return c.client.AddTorrent(metaInfo)
	})
}

// addTorrent runs the given add function after checking the torrent is
// not already managed, records its metadata and starts driving it.
func (c *client) addTorrent(infoHash metainfo.Hash, category string,
	add func() (*torrent.Torrent, error),
) (infoHashHex string, err error) {
	infoHashHex = infoHash.HexString()
	if _, exists := c.client.Torrent(infoHash); exists {
		return infoHashHex, fmt.Errorf("%w: %s", ErrTorrentExists, infoHashHex)
	}

	addedTorrent, err := add()
	if err != nil {
		return "", err
	}

	c.setMeta(infoHashHex, torrentMeta{
		category: category,
		addedAt:  time.Now().Unix(),
	})

	c.drive(addedTorrent)
	return infoHashHex, nil
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
		list = append(list, c.torrentInfo(torrent))
	}
	return list
}

func (c *client) torrentInfo(t *torrent.Torrent) (info models.BittorrentTorrent) {
	stats := t.Stats()
	info = models.BittorrentTorrent{
		InfoHash:    t.InfoHash().HexString(),
		Name:        t.Name(),
		SavePath:    c.downloadDir,
		ContentPath: filepath.Join(c.downloadDir, t.Name()),
		NumLeechs:   stats.TotalPeers - stats.ConnectedSeeders,
		NumSeeds:    stats.ConnectedSeeders,
		Downloaded:  stats.BytesReadData.Int64(),
		Uploaded:    stats.BytesWrittenData.Int64(),
		TimeAdded:   c.getMeta(t.InfoHash().HexString()).addedAt,
		Category:    c.getMeta(t.InfoHash().HexString()).category,
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

	files := t.Files()
	info.Files = make([]models.BittorrentFile, 0, len(files))
	for i, file := range files {
		info.Files = append(info.Files, models.BittorrentFile{
			Index:          i,
			Name:           file.Path(),
			Length:         file.Length(),
			BytesCompleted: file.BytesCompleted(),
		})
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
		return fmt.Errorf("%w: %s", ErrTorrentNotFound, infoHashHex)
	}

	torrent.Drop()
	c.deleteMeta(infoHashHex)
	return nil
}
