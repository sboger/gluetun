package server

import (
	"encoding/json"
	"net/http"

	"github.com/qdm12/gluetun/internal/models"
)

func newBittorrentHandler(bittorrent Bittorrent, warner warner) http.Handler {
	return &bittorrentHandler{
		bittorrent: bittorrent,
		warner:     warner,
	}
}

type bittorrentHandler struct {
	bittorrent Bittorrent
	warner     warner
}

func (h *bittorrentHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		h.listTorrents(w)
	case http.MethodPost:
		h.addTorrent(w, r)
	case http.MethodDelete:
		h.removeTorrent(w, r)
	default:
		errMethodNotSupported(w, r.Method)
	}
}

func (h *bittorrentHandler) listTorrents(w http.ResponseWriter) {
	torrents := h.bittorrent.ListTorrents()
	if torrents == nil {
		torrents = []models.BittorrentTorrent{}
	}
	encoder := json.NewEncoder(w)
	if err := encoder.Encode(torrents); err != nil {
		h.warner.Warn(err.Error())
		w.WriteHeader(http.StatusInternalServerError)
	}
}

func (h *bittorrentHandler) addTorrent(w http.ResponseWriter, r *http.Request) {
	var data bittorrentAddRequest
	decoder := json.NewDecoder(r.Body)
	if err := decoder.Decode(&data); err != nil {
		h.warner.Warn("failed adding torrent: " + err.Error())
		http.Error(w, "failed adding torrent: invalid JSON body", http.StatusBadRequest)
		return
	}

	if data.Magnet == "" {
		http.Error(w, "failed adding torrent: magnet URI is required", http.StatusBadRequest)
		return
	}

	infoHash, err := h.bittorrent.AddTorrent(data.Magnet)
	if err != nil {
		h.warner.Warn("failed adding torrent: " + err.Error())
		http.Error(w, "failed adding torrent: "+err.Error(), http.StatusBadRequest)
		return
	}

	encoder := json.NewEncoder(w)
	if err := encoder.Encode(models.BittorrentAddResult{InfoHash: infoHash}); err != nil {
		h.warner.Warn(err.Error())
		w.WriteHeader(http.StatusInternalServerError)
	}
}

func (h *bittorrentHandler) removeTorrent(w http.ResponseWriter, r *http.Request) {
	var data bittorrentRemoveRequest
	decoder := json.NewDecoder(r.Body)
	if err := decoder.Decode(&data); err != nil {
		h.warner.Warn("failed removing torrent: " + err.Error())
		http.Error(w, "failed removing torrent: invalid JSON body", http.StatusBadRequest)
		return
	}

	if data.InfoHash == "" {
		http.Error(w, "failed removing torrent: info hash is required", http.StatusBadRequest)
		return
	}

	if err := h.bittorrent.RemoveTorrent(data.InfoHash); err != nil {
		h.warner.Warn("failed removing torrent: " + err.Error())
		http.Error(w, "failed removing torrent: "+err.Error(), http.StatusBadRequest)
		return
	}

	w.WriteHeader(http.StatusOK)
}

type bittorrentAddRequest struct {
	Magnet string `json:"magnet"`
}

type bittorrentRemoveRequest struct {
	InfoHash string `json:"info_hash"`
}
