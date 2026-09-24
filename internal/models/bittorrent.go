package models

// BittorrentTorrent is the public representation of a torrent
// managed by the embedded BitTorrent client, used to expose the
// active torrent list through the control server API.
type BittorrentTorrent struct {
	// InfoHash is the torrent infohash, hex encoded.
	InfoHash string `json:"info_hash"`
	// Name is the torrent display name.
	Name string `json:"name"`
	// Length is the total length of the torrent data in bytes.
	// It is 0 while the metadata is not yet available.
	Length int64 `json:"length"`
	// BytesCompleted is the number of bytes downloaded so far.
	BytesCompleted int64 `json:"bytes_completed"`
	// PercentComplete is the download completion percentage, 0 to 100.
	// It is 0 while the metadata is not yet available.
	PercentComplete int `json:"percent_complete"`
	// State is the current torrent state, for example
	// "fetching metadata", "downloading" or "seeding".
	State string `json:"state"`
	// Seeding is true if the torrent is currently being seeded.
	Seeding bool `json:"seeding"`
}

// BittorrentAddResult is the response of adding a torrent.
type BittorrentAddResult struct {
	// InfoHash is the hex encoded torrrent info hash.
	InfoHash string `json:"info_hash"`
}
