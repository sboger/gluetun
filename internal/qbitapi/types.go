package qbitapi

// qbitTorrentInfo mirrors the object qBittorrent returns from
// GET /api/v2/torrents/info. Field names and JSON tags match the
// qBittorrent Web API so Sonarr/Radarr can parse them unchanged.
type qbitTorrentInfo struct {
	Hash                string  `json:"hash"`
	Name                string  `json:"name"`
	Size                int64   `json:"size"`
	Progress            float64 `json:"progress"`
	Downloaded          int64   `json:"downloaded"`
	Uploaded            int64   `json:"uploaded"`
	AmountLeft          int64   `json:"amount_left"`
	ETA                 int64   `json:"eta"`
	State               string  `json:"state"`
	DownloadSpeed       int64   `json:"dlspeed"`
	UploadSpeed         int64   `json:"upspeed"`
	Ratio               float64 `json:"ratio"`
	Category            string  `json:"category"`
	Label               string  `json:"label"`
	SavePath            string  `json:"save_path"`
	ContentPath         string  `json:"content_path"`
	NumSeeds            int     `json:"num_seeds"`
	NumLeechs           int     `json:"num_leechs"`
	TimeAdded           int64   `json:"added_on"`
	SeedingTime         int64   `json:"seeding_time"`
	LastActivity        int64   `json:"last_activity"`
	RatioLimit          int     `json:"ratio_limit"`
	SeedingTimeLimit    int     `json:"seeding_time_limit"`
	InactiveSeedingTime int     `json:"inactive_seeding_time_limit"`
	TotalSeeds          int     `json:"num_complete"`
	TotalLeechs         int     `json:"num_incomplete"`
}

// qbitTorrentProperties mirrors GET /api/v2/torrents/properties.
type qbitTorrentProperties struct {
	SavePath        string  `json:"save_path"`
	CreationDate    int64   `json:"creation_date"`
	PieceSize       int64   `json:"piece_size"`
	Comment         string  `json:"comment"`
	TotalWasted     int64   `json:"total_wasted"`
	TotalUploaded   int64   `json:"total_uploaded"`
	TotalDownloaded int64   `json:"total_downloaded"`
	UploadRatio     float64 `json:"upload_ratio"`
	DownloadPath    string  `json:"download_path"`
	AddedOn         int64   `json:"added_on"`
	CompletedOn     int64   `json:"completed_on"`
	FileCount       int     `json:"file_count"`
	Hash            string  `json:"hash"`
	Name            string  `json:"name"`
	Size            int64   `json:"size"`
	Progress        float64 `json:"progress"`
	SeedingTime     int64   `json:"seeding_time"`
}

// qbitTorrentFile mirrors one entry of GET /api/v2/torrents/files.
type qbitTorrentFile struct {
	Index    int     `json:"index"`
	Name     string  `json:"name"`
	Size     int64   `json:"size"`
	Progress float64 `json:"progress"`
	Priority int     `json:"priority"`
	IsSeed   int     `json:"is_seed"`
}

// qbitPreferences mirrors the subset of GET /api/v2/app/preferences
// that Sonarr/Radarr read.
type qbitPreferences struct {
	DhtEnabled                    bool    `json:"dht"`
	SavePath                      string  `json:"save_path"`
	MaxRatioEnabled               bool    `json:"max_ratio_enabled"`
	MaxRatio                      float64 `json:"max_ratio"`
	MaxSeedingTimeEnabled         bool    `json:"max_seeding_time_enabled"`
	MaxSeedingTime                int64   `json:"max_seeding_time"`
	MaxInactiveSeedingTimeEnabled bool    `json:"max_inactive_seeding_time_enabled"`
	MaxInactiveSeedingTime        int64   `json:"max_inactive_seeding_time"`
}

// qbitCategory is an entry of GET /api/v2/torrents/categories.
type qbitCategory struct {
	Name             string `json:"name"`
	SavePath         string `json:"savePath"`
	IsSubCategorized bool   `json:"isSubCategorized"`
}

// qbitBitness is the reported platform word size in qBittorrent's
// buildInfo payload.
const qbitBitness = 64

// qbitBuildInfo mirrors GET /api/v2/app/buildInfo.
func qbitBuildInfo() map[string]any {
	return map[string]any{
		"qt":         "6.4.2",
		"libtorrent": "2.0.9.0",
		"boost":      "1.80.0",
		"openssl":    "3.0.8",
		"zlib":       "1.2.13",
		"bitness":    qbitBitness,
		"platform":   "linux-x86_64",
	}
}
