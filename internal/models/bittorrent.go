package models

// BittorrentTorrent is the public representation of a torrent
// managed by the embedded BitTorrent client, used to expose the
// active torrent list through the control server API and the
// qBittorrent-compatible API.
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
	// Category is an optional category tag assigned when the torrent
	// was added, typically set by the download client (e.g. Sonarr).
	Category string `json:"category"`
	// TimeAdded is the Unix timestamp (seconds) at which the torrent
	// was added to the client. It is 0 for torrents added before the
	// category tracking was introduced (none at runtime).
	TimeAdded int64 `json:"time_added"`
	// Downloaded is the total number of payload bytes downloaded from
	// peers so far (excluding protocol overhead).
	Downloaded int64 `json:"downloaded"`
	// Uploaded is the total number of payload bytes uploaded to peers
	// so far (excluding protocol overhead).
	Uploaded int64 `json:"uploaded"`
	// NumSeeds is the number of connected seeders.
	NumSeeds int `json:"num_seeds"`
	// NumLeechs is the number of connected leechers.
	NumLeechs int `json:"num_leechs"`
	// SavePath is the directory the torrent data is stored in.
	SavePath string `json:"save_path"`
	// ContentPath is the path of the torrent content on disk: the
	// data directory for a multi-file torrent, or the file path for a
	// single-file torrent.
	ContentPath string `json:"content_path"`
	// Files lists the files of the torrent. It is nil while the
	// metadata is not yet available.
	Files []BittorrentFile `json:"files"`
}

// BittorrentFile is a file inside a torrent.
type BittorrentFile struct {
	// Index is the zero-based index of the file within the torrent.
	Index int `json:"index"`
	// Name is the relative path of the file within the torrent.
	Name string `json:"name"`
	// Length is the size of the file in bytes.
	Length int64 `json:"length"`
	// BytesCompleted is the number of bytes of the file downloaded so far.
	BytesCompleted int64 `json:"bytes_completed"`
}

// BittorrentAddResult is the response of adding a torrent.
type BittorrentAddResult struct {
	// InfoHash is the hex encoded torrrent info hash.
	InfoHash string `json:"info_hash"`
}
