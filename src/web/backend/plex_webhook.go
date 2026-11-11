package backend

import (
	"crypto/subtle"
	"encoding/json"
	"log/slog"
	"net/http"
	"strings"

	"github.com/ilyakaznacheev/cleanenv"

	"explo/src/config"
	"explo/src/downloader"
	"explo/src/models"
)

type plexWebhook struct {
	Event    string `json:"event"`
	Metadata struct {
		Type                string  `json:"type"`
		LibrarySectionTitle string  `json:"librarySectionTitle"`
		Title               string  `json:"title"`
		ParentTitle         string  `json:"parentTitle"`      // album
		GrandparentTitle    string  `json:"grandparentTitle"` // album artist
		UserRating          float64 `json:"userRating"`
	} `json:"Metadata"`
}

// ratedTrack returns the track a Plex webhook payload rated, or nil when the
// event isn't a (positive) rating of a track in the given library.
func ratedTrack(payload, library string) *models.Track {
	var p plexWebhook
	if json.Unmarshal([]byte(payload), &p) != nil {
		return nil
	}
	m := p.Metadata
	if p.Event != "media.rate" || m.Type != "track" || m.UserRating <= 0 ||
		!strings.EqualFold(m.LibrarySectionTitle, library) {
		return nil
	}
	return &models.Track{Title: m.Title, CleanTitle: m.Title, Album: m.ParentTitle, Artist: m.GrandparentTitle, MainArtist: m.GrandparentTitle}
}

// handlePlexWebhook requests the album of a track rated in the Explo library from Lidarr.
// Plex can't send auth headers, so the endpoint is guarded by PLEX_WEBHOOK_TOKEN in the URL.
func (s *Server) handlePlexWebhook(w http.ResponseWriter, r *http.Request) {
	var cfg config.Config // re-read so settings changed in the UI apply without a restart
	if err := cleanenv.ReadConfig(s.cfg.WebEnvPath, &cfg); err != nil {
		slog.Error("plex webhook: failed to load config", "err", err.Error())
		http.Error(w, "config error", http.StatusInternalServerError)
		return
	}
	cfg.CommonFixes()

	token := cfg.ServerCfg.PlexWebhookToken
	if token == "" {
		http.NotFound(w, r)
		return
	}
	if subtle.ConstantTimeCompare([]byte(r.URL.Query().Get("token")), []byte(token)) != 1 {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, 1<<20) // payload may carry a thumbnail
	track := ratedTrack(r.FormValue("payload"), cfg.ClientCfg.LibraryName)
	w.WriteHeader(http.StatusOK)
	if track == nil {
		return
	}

	slog.Info("plex webhook: track rated, requesting album from Lidarr", "artist", track.MainArtist, "album", track.Album, "track", track.Title)
	go func() { // adding an album waits on Lidarr's search, don't hold Plex's request open
		lidarr := downloader.NewLidarr(cfg.DownloadCfg.Lidarr, cfg.DownloadCfg.DownloadDir)
		if err := lidarr.RequestAlbum(track); err != nil {
			slog.Error("plex webhook: lidarr request failed", "artist", track.MainArtist, "album", track.Album, "err", err.Error())
			return
		}
		slog.Info("plex webhook: album requested in Lidarr", "artist", track.MainArtist, "album", track.Album)
	}()
}
