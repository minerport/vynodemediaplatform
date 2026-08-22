package httpserver

import (
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/vynode/media/server/internal/auth"
	"github.com/vynode/media/server/internal/playback"
)

func (h *Handler) playbackRoutes(mux *http.ServeMux) {
	mux.HandleFunc("POST /api/v1/playback/sessions", h.require(auth.CapPlaybackStart, h.startPlayback))
	mux.HandleFunc("PATCH /api/v1/playback/sessions/{sessionId}", h.require(auth.CapPlaybackSelfManage, h.updatePlayback))
	mux.HandleFunc("DELETE /api/v1/playback/sessions/{sessionId}", h.require(auth.CapPlaybackSelfManage, h.stopPlayback))
	mux.HandleFunc("GET /api/v1/playback/{logicalType}/{logicalId}/versions", h.require(auth.CapPlaybackStart, h.playbackVersions))
	mux.HandleFunc("GET /api/v1/playback/{logicalType}/{logicalId}/progress", h.require(auth.CapPlaybackSelfManage, h.playbackProgress))
	mux.HandleFunc("PUT /api/v1/playback/{logicalType}/{logicalId}/watched", h.require(auth.CapPlaybackSelfManage, h.markWatched))
	mux.HandleFunc("GET /api/v1/admin/playback/sessions", h.require(auth.CapPlaybackSessionsView, h.activePlayback))
	mux.HandleFunc("DELETE /api/v1/admin/playback/sessions/{sessionId}", h.require(auth.CapPlaybackSessionsManage, h.adminStopPlayback))
	mux.HandleFunc("GET /api/v1/playback/sessions/{sessionId}/media", h.playbackMedia)
	mux.HandleFunc("HEAD /api/v1/playback/sessions/{sessionId}/media", h.playbackMedia)
	mux.HandleFunc("GET /api/v1/playback/sessions/{sessionId}/subtitles/{trackId}", h.playbackSubtitle)
	mux.HandleFunc("GET /api/v1/playback/continue-watching", h.require(auth.CapPlaybackSelfManage, h.continueWatching))
	mux.HandleFunc("GET /api/v1/admin/playback/capabilities", h.require(auth.CapPlaybackSessionsView, h.playbackCapabilities))
}

func (h *Handler) startPlayback(w http.ResponseWriter, r *http.Request, p auth.Principal) {
	var in playback.StartRequest
	if !decode(w, r, &in) {
		return
	}
	out, err := h.playback.Start(r.Context(), p.UserID, p.SessionID, in)
	if err != nil {
		h.playbackError(w, r, err)
		return
	}
	if out.MediaURL != "" {
		parts := strings.SplitN(out.MediaURL, "?token=", 2)
		if len(parts) == 2 {
			secure := r.TLS != nil
			http.SetCookie(w, &http.Cookie{Name: mediaCookie(out.ID), Value: parts[1], Path: "/api/v1/playback/sessions/" + out.ID + "/", HttpOnly: true, Secure: secure, SameSite: http.SameSiteStrictMode, MaxAge: int((4 * time.Hour).Seconds())})
			out.MediaURL = parts[0]
		}
	}
	writeJSON(w, 201, out)
}
func mediaCookie(id string) string { return "vynode_media_" + strings.ReplaceAll(id, "-", "") }
func (h *Handler) updatePlayback(w http.ResponseWriter, r *http.Request, p auth.Principal) {
	var in playback.Progress
	if !decode(w, r, &in) {
		return
	}
	if err := h.playback.Update(r.Context(), p.UserID, r.PathValue("sessionId"), in); err != nil {
		h.playbackError(w, r, err)
		return
	}
	w.WriteHeader(204)
}
func (h *Handler) stopPlayback(w http.ResponseWriter, r *http.Request, p auth.Principal) {
	if err := h.playback.Stop(r.Context(), p.UserID, r.PathValue("sessionId")); err != nil {
		h.playbackError(w, r, err)
		return
	}
	w.WriteHeader(204)
}
func (h *Handler) playbackVersions(w http.ResponseWriter, r *http.Request, _ auth.Principal) {
	v, err := h.playback.Versions(r.Context(), strings.ToUpper(r.PathValue("logicalType")), r.PathValue("logicalId"))
	if err != nil {
		h.playbackError(w, r, err)
		return
	}
	writeJSON(w, 200, map[string]any{"versions": v})
}
func (h *Handler) playbackProgress(w http.ResponseWriter, r *http.Request, p auth.Principal) {
	v, err := h.playback.Progress(r.Context(), p.UserID, strings.ToUpper(r.PathValue("logicalType")), r.PathValue("logicalId"))
	if err != nil {
		h.playbackError(w, r, err)
		return
	}
	writeJSON(w, 200, v)
}
func (h *Handler) markWatched(w http.ResponseWriter, r *http.Request, p auth.Principal) {
	var in struct {
		Watched bool `json:"watched"`
	}
	if !decode(w, r, &in) {
		return
	}
	if err := h.playback.MarkWatched(r.Context(), p.UserID, strings.ToUpper(r.PathValue("logicalType")), r.PathValue("logicalId"), in.Watched); err != nil {
		h.playbackError(w, r, err)
		return
	}
	w.WriteHeader(204)
}
func (h *Handler) activePlayback(w http.ResponseWriter, r *http.Request, _ auth.Principal) {
	v, err := h.playback.Active(r.Context())
	if err != nil {
		h.playbackError(w, r, err)
		return
	}
	writeJSON(w, 200, map[string]any{"sessions": v})
}
func (h *Handler) adminStopPlayback(w http.ResponseWriter, r *http.Request, p auth.Principal) {
	id := r.PathValue("sessionId")
	if err := h.playback.AdminStop(r.Context(), id); err != nil {
		h.playbackError(w, r, err)
		return
	}
	_ = h.auth.Audit(r.Context(), "PLAYBACK_SESSION_TERMINATED", &p.UserID, "playback_session", id, RequestID(r.Context()), map[string]any{"outcome": "success"})
	w.WriteHeader(204)
}
func (h *Handler) playbackError(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, playback.ErrValidation):
		writeError(w, r, 400, "VALIDATION_FAILED", "The playback request is invalid.")
	case errors.Is(err, playback.ErrForbidden):
		writeError(w, r, 403, "PLAYBACK_FORBIDDEN", "Playback authorization was denied.")
	case errors.Is(err, playback.ErrUnavailable):
		writeError(w, r, 409, "MEDIA_UNAVAILABLE", "The media source is unavailable.")
	case errors.Is(err, playback.ErrStale):
		writeError(w, r, 409, "STALE_INVENTORY", "The media file changed and must be rescanned.")
	case errors.Is(err, playback.ErrExpired):
		writeError(w, r, 410, "PLAYBACK_SESSION_ENDED", "The playback session is no longer active.")
	case errors.Is(err, playback.ErrNotFound):
		writeError(w, r, 404, "NOT_FOUND", "The requested logical media was not found.")
	case errors.Is(err, playback.ErrCapacity):
		writeError(w, r, 503, "PLAYBACK_CAPACITY_REACHED", "All generated playback pipelines are currently in use.")
	case errors.Is(err, playback.ErrPipelineUnavailable):
		writeError(w, r, 503, "FFMPEG_UNAVAILABLE", "Generated playback is unavailable on this server.")
	default:
		writeError(w, r, 500, "INTERNAL_ERROR", "An unexpected playback error occurred.")
	}
}

func (h *Handler) playbackMedia(w http.ResponseWriter, r *http.Request) {
	token := ""
	if c, e := r.Cookie(mediaCookie(r.PathValue("sessionId"))); e == nil {
		token = c.Value
	}
	if bearer(r) != "" {
		p, e := h.auth.Authenticate(bearer(r))
		if e != nil || !h.playback.OwnedBy(r.Context(), r.PathValue("sessionId"), p.UserID, p.SessionID) {
			h.playbackError(w, r, playback.ErrForbidden)
			return
		}
	}
	a, err := h.playback.AuthorizeMedia(r.Context(), r.PathValue("sessionId"), token)
	if err != nil {
		h.playbackError(w, r, err)
		return
	}
	if a.Mode != playback.DirectPlay {
		if r.Header.Get("Range") != "" {
			w.Header().Set("Content-Range", "bytes */*")
			w.WriteHeader(http.StatusRequestedRangeNotSatisfiable)
			return
		}
		start := 0.0
		if raw := r.URL.Query().Get("start"); raw != "" {
			start, err = strconv.ParseFloat(raw, 64)
			if err != nil || start < 0 || start > 7*24*3600 {
				h.playbackError(w, r, playback.ErrValidation)
				return
			}
		}
		w.Header().Set("Content-Type", "video/mp4")
		w.Header().Set("Cache-Control", "no-store")
		w.Header().Set("X-VyNode-Playback-Mode", string(a.Mode))
		w.WriteHeader(http.StatusOK)
		if r.Method != "HEAD" {
			_ = h.playback.StreamGenerated(r.Context(), a, start, w)
		}
		return
	}
	f, err := os.Open(a.Path)
	if err != nil {
		h.playbackError(w, r, playback.ErrUnavailable)
		return
	}
	defer f.Close()
	w.Header().Set("Accept-Ranges", "bytes")
	w.Header().Set("Content-Type", a.MIME)
	w.Header().Set("ETag", a.ETag)
	w.Header().Set("Last-Modified", a.Modified)
	if r.Header.Get("Range") == "" {
		w.Header().Set("Content-Length", strconv.FormatInt(a.Size, 10))
		w.WriteHeader(200)
		if r.Method != "HEAD" {
			n, _ := io.Copy(w, f)
			h.playback.AddBytes(r.Context(), a.SessionID, n)
		}
		return
	}
	start, end, ok := singleRange(r.Header.Get("Range"), a.Size)
	if !ok {
		w.Header().Set("Content-Range", fmt.Sprintf("bytes */%d", a.Size))
		w.WriteHeader(http.StatusRequestedRangeNotSatisfiable)
		return
	}
	length := end - start + 1
	w.Header().Set("Content-Length", strconv.FormatInt(length, 10))
	w.Header().Set("Content-Range", fmt.Sprintf("bytes %d-%d/%d", start, end, a.Size))
	w.WriteHeader(http.StatusPartialContent)
	if r.Method != "HEAD" {
		n, _ := io.CopyN(w, io.NewSectionReader(f, start, length), length)
		h.playback.AddBytes(r.Context(), a.SessionID, n)
	}
}

func (h *Handler) playbackSubtitle(w http.ResponseWriter, r *http.Request) {
	token := ""
	if c, e := r.Cookie(mediaCookie(r.PathValue("sessionId"))); e == nil {
		token = c.Value
	}
	b, e := h.playback.Subtitle(r.Context(), r.PathValue("sessionId"), token, r.PathValue("trackId"))
	if e != nil {
		h.playbackError(w, r, e)
		return
	}
	w.Header().Set("Content-Type", "text/vtt; charset=utf-8")
	w.Header().Set("Cache-Control", "private, no-store")
	w.WriteHeader(200)
	_, _ = w.Write(b)
}
func (h *Handler) continueWatching(w http.ResponseWriter, r *http.Request, p auth.Principal) {
	v, e := h.playback.ContinueWatching(r.Context(), p.UserID)
	if e != nil {
		h.playbackError(w, r, e)
		return
	}
	writeJSON(w, 200, map[string]any{"items": v})
}
func (h *Handler) playbackCapabilities(w http.ResponseWriter, r *http.Request, _ auth.Principal) {
	writeJSON(w, 200, h.playback.Capabilities())
}
func singleRange(value string, size int64) (int64, int64, bool) {
	if size <= 0 || !strings.HasPrefix(value, "bytes=") {
		return 0, 0, false
	}
	spec := strings.TrimSpace(strings.TrimPrefix(value, "bytes="))
	if strings.Contains(spec, ",") {
		return 0, 0, false
	}
	parts := strings.Split(spec, "-")
	if len(parts) != 2 {
		return 0, 0, false
	}
	if parts[0] == "" {
		n, e := strconv.ParseInt(parts[1], 10, 64)
		if e != nil || n <= 0 {
			return 0, 0, false
		}
		if n > size {
			n = size
		}
		return size - n, size - 1, true
	}
	start, e := strconv.ParseInt(parts[0], 10, 64)
	if e != nil || start < 0 || start >= size {
		return 0, 0, false
	}
	end := size - 1
	if parts[1] != "" {
		end, e = strconv.ParseInt(parts[1], 10, 64)
		if e != nil || end < start {
			return 0, 0, false
		}
		if end >= size {
			end = size - 1
		}
	}
	return start, end, true
}
