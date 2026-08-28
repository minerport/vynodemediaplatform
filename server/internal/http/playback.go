package httpserver

import (
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
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
	mux.HandleFunc("GET /api/v1/playback/sessions/{sessionId}/hls/{file}", h.playbackHLS)
	mux.HandleFunc("GET /api/v1/playback/sessions/{sessionId}/subtitles/{trackId}", h.playbackSubtitle)
	mux.HandleFunc("GET /api/v1/playback/continue-watching", h.require(auth.CapPlaybackSelfManage, h.continueWatching))
	mux.HandleFunc("GET /api/v1/admin/playback/capabilities", h.require(auth.CapPlaybackSessionsView, h.playbackCapabilities))
	mux.HandleFunc("GET /api/v1/account/playback-preferences", h.require(auth.CapPlaybackSelfManage, h.getPlaybackPreferences))
	mux.HandleFunc("PATCH /api/v1/account/playback-preferences", h.require(auth.CapPlaybackSelfManage, h.setPlaybackPreferences))
	mux.HandleFunc("GET /api/v1/playback/{logicalType}/{logicalId}/markers", h.require(auth.CapPlaybackStart, h.playbackMarkers))
	mux.HandleFunc("POST /api/v1/admin/media-markers", h.require(auth.CapMetadataManage, h.createMarker))
	mux.HandleFunc("PATCH /api/v1/admin/media-markers/{markerId}", h.require(auth.CapMetadataManage, h.updateMarker))
	mux.HandleFunc("DELETE /api/v1/admin/media-markers/{markerId}", h.require(auth.CapMetadataManage, h.deleteMarker))
	mux.HandleFunc("DELETE /api/v1/playback/continue-watching/items/{logicalType}/{logicalId}", h.require(auth.CapPlaybackSelfManage, h.dismissContinueWatching))
	mux.HandleFunc("POST /api/v1/playback/{logicalType}/{logicalId}/start-over", h.require(auth.CapPlaybackSelfManage, h.startOver))
}

func (h *Handler) getPlaybackPreferences(w http.ResponseWriter, r *http.Request, p auth.Principal) {
	v, e := h.playback.Preferences(r.Context(), p.UserID)
	if e != nil {
		h.playbackError(w, r, e)
		return
	}
	writeJSON(w, 200, v)
}
func (h *Handler) setPlaybackPreferences(w http.ResponseWriter, r *http.Request, p auth.Principal) {
	var in playback.PlaybackPreferences
	if !decode(w, r, &in) {
		return
	}
	v, e := h.playback.SetPreferences(r.Context(), p.UserID, in)
	if e != nil {
		h.playbackError(w, r, e)
		return
	}
	writeJSON(w, 200, v)
}
func (h *Handler) playbackMarkers(w http.ResponseWriter, r *http.Request, p auth.Principal) {
	if !h.sharing.HasLogical(r.Context(), p, strings.ToUpper(r.PathValue("logicalType")), r.PathValue("logicalId"), "VIEW") {
		h.playbackError(w, r, playback.ErrNotFound)
		return
	}
	v, e := h.playback.Markers(r.Context(), strings.ToUpper(r.PathValue("logicalType")), r.PathValue("logicalId"))
	if e != nil {
		h.playbackError(w, r, e)
		return
	}
	writeJSON(w, 200, map[string]any{"markers": v})
}
func (h *Handler) createMarker(w http.ResponseWriter, r *http.Request, p auth.Principal) {
	var in playback.Marker
	if !decode(w, r, &in) {
		return
	}
	v, e := h.playback.SaveMarker(r.Context(), in)
	if e != nil {
		h.playbackError(w, r, e)
		return
	}
	_ = h.auth.Audit(r.Context(), "MEDIA_MARKER_CREATED", &p.UserID, "media_marker", v.ID, RequestID(r.Context()), map[string]any{"logicalType": v.LogicalType, "logicalId": v.LogicalID, "markerType": v.Type})
	writeJSON(w, 201, v)
}
func (h *Handler) updateMarker(w http.ResponseWriter, r *http.Request, p auth.Principal) {
	var in playback.Marker
	if !decode(w, r, &in) {
		return
	}
	in.ID = r.PathValue("markerId")
	v, e := h.playback.SaveMarker(r.Context(), in)
	if e != nil {
		h.playbackError(w, r, e)
		return
	}
	_ = h.auth.Audit(r.Context(), "MEDIA_MARKER_UPDATED", &p.UserID, "media_marker", v.ID, RequestID(r.Context()), map[string]any{"markerType": v.Type})
	writeJSON(w, 200, v)
}
func (h *Handler) deleteMarker(w http.ResponseWriter, r *http.Request, p auth.Principal) {
	markerID := r.PathValue("markerId")
	if e := h.playback.DeleteMarker(r.Context(), markerID); e != nil {
		h.playbackError(w, r, e)
		return
	}
	_ = h.auth.Audit(r.Context(), "MEDIA_MARKER_DELETED", &p.UserID, "media_marker", markerID, RequestID(r.Context()), map[string]any{})
	w.WriteHeader(204)
}
func (h *Handler) dismissContinueWatching(w http.ResponseWriter, r *http.Request, p auth.Principal) {
	if !h.sharing.HasLogical(r.Context(), p, strings.ToUpper(r.PathValue("logicalType")), r.PathValue("logicalId"), "VIEW") {
		h.playbackError(w, r, playback.ErrNotFound)
		return
	}
	if e := h.playback.DismissContinue(r.Context(), p.UserID, strings.ToUpper(r.PathValue("logicalType")), r.PathValue("logicalId")); e != nil {
		h.playbackError(w, r, e)
		return
	}
	w.WriteHeader(204)
}
func (h *Handler) startOver(w http.ResponseWriter, r *http.Request, p auth.Principal) {
	if !h.sharing.HasLogical(r.Context(), p, strings.ToUpper(r.PathValue("logicalType")), r.PathValue("logicalId"), "VIEW") {
		h.playbackError(w, r, playback.ErrNotFound)
		return
	}
	if e := h.playback.ResetProgress(r.Context(), p.UserID, strings.ToUpper(r.PathValue("logicalType")), r.PathValue("logicalId")); e != nil {
		h.playbackError(w, r, e)
		return
	}
	w.WriteHeader(204)
}

func (h *Handler) startPlayback(w http.ResponseWriter, r *http.Request, p auth.Principal) {
	var in playback.StartRequest
	if !decode(w, r, &in) {
		return
	}
	if !h.sharing.HasLogical(r.Context(), p, in.LogicalType, in.LogicalID, "PLAY") {
		h.playbackError(w, r, playback.ErrNotFound)
		return
	}
	in.Network = playback.NetworkRemote
	_, _, local := h.connection(r)
	if local {
		in.Network = playback.NetworkLocal
	}
	out, err := h.playback.Start(r.Context(), p.UserID, p.SessionID, in)
	if err != nil {
		h.playbackError(w, r, err)
		return
	}
	nativeClient := strings.EqualFold(strings.TrimSpace(r.Header.Get("X-VyNode-Client")), "native")
	if out.MediaURL != "" && !nativeClient {
		parts := strings.SplitN(out.MediaURL, "?token=", 2)
		if len(parts) == 2 {
			_, secure, _ := h.connection(r)
			http.SetCookie(w, &http.Cookie{Name: mediaCookie(out.ID), Value: parts[1], Path: "/api/v1/playback/sessions/" + out.ID + "/", HttpOnly: true, Secure: secure, SameSite: http.SameSiteStrictMode, MaxAge: int((4 * time.Hour).Seconds())})
			out.MediaURL = parts[0]
		}
	}
	if out.HLSURL != "" && !nativeClient {
		parts := strings.SplitN(out.HLSURL, "?token=", 2)
		if len(parts) == 2 {
			_, secure, _ := h.connection(r)
			http.SetCookie(w, &http.Cookie{Name: mediaCookie(out.ID), Value: parts[1], Path: "/api/v1/playback/sessions/" + out.ID + "/", HttpOnly: true, Secure: secure, SameSite: http.SameSiteStrictMode, MaxAge: int((4 * time.Hour).Seconds())})
			out.HLSURL = parts[0]
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
func (h *Handler) playbackVersions(w http.ResponseWriter, r *http.Request, p auth.Principal) {
	if !h.sharing.HasLogical(r.Context(), p, strings.ToUpper(r.PathValue("logicalType")), r.PathValue("logicalId"), "VIEW") {
		h.playbackError(w, r, playback.ErrNotFound)
		return
	}
	v, err := h.playback.Versions(r.Context(), strings.ToUpper(r.PathValue("logicalType")), r.PathValue("logicalId"))
	if err != nil {
		h.playbackError(w, r, err)
		return
	}
	if p.Role == auth.RoleUser {
		filtered := v[:0]
		for _, version := range v {
			if h.sharing.HasAssociation(r.Context(), p, version.ID, "VIEW") {
				filtered = append(filtered, version)
			}
		}
		v = filtered
	}
	writeJSON(w, 200, map[string]any{"versions": v})
}
func (h *Handler) playbackProgress(w http.ResponseWriter, r *http.Request, p auth.Principal) {
	if !h.sharing.HasLogical(r.Context(), p, strings.ToUpper(r.PathValue("logicalType")), r.PathValue("logicalId"), "VIEW") {
		h.playbackError(w, r, playback.ErrNotFound)
		return
	}
	v, err := h.playback.Progress(r.Context(), p.UserID, strings.ToUpper(r.PathValue("logicalType")), r.PathValue("logicalId"))
	if err != nil {
		h.playbackError(w, r, err)
		return
	}
	writeJSON(w, 200, v)
}
func (h *Handler) markWatched(w http.ResponseWriter, r *http.Request, p auth.Principal) {
	if !h.sharing.HasLogical(r.Context(), p, strings.ToUpper(r.PathValue("logicalType")), r.PathValue("logicalId"), "VIEW") {
		h.playbackError(w, r, playback.ErrNotFound)
		return
	}
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
	case errors.Is(err, playback.ErrVideoCapacity):
		writeError(w, r, 503, "TRANSCODE_CAPACITY_REACHED", "All video transcode slots are currently in use.")
	case errors.Is(err, playback.ErrTranscodeStorage):
		writeError(w, r, 507, "TRANSCODE_STORAGE_EXHAUSTED", "Transcode storage is unavailable.")
	case errors.Is(err, playback.ErrPipelineUnavailable):
		writeError(w, r, 503, "FFMPEG_UNAVAILABLE", "Generated playback is unavailable on this server.")
	default:
		writeError(w, r, 500, "INTERNAL_ERROR", "An unexpected playback error occurred.")
	}
}

func (h *Handler) playbackHLS(w http.ResponseWriter, r *http.Request) {
	token := mediaAccessToken(r)
	var p string
	var e error
	if raw := bearer(r); raw != "" {
		principal, authErr := h.auth.Authenticate(raw)
		if authErr != nil || !h.playback.OwnedBy(r.Context(), r.PathValue("sessionId"), principal.UserID, principal.SessionID) {
			h.playbackError(w, r, playback.ErrForbidden)
			return
		}
		p, e = h.playback.HLSFileForOwner(r.Context(), r.PathValue("sessionId"), r.PathValue("file"))
	} else {
		p, e = h.playback.HLSFile(r.Context(), r.PathValue("sessionId"), token, r.PathValue("file"))
	}
	if e != nil {
		h.playbackError(w, r, e)
		return
	}
	if strings.HasSuffix(p, ".m3u8") {
		w.Header().Set("Content-Type", "application/vnd.apple.mpegurl")
		if token != "" && bearer(r) == "" {
			body, readErr := os.ReadFile(p)
			if readErr != nil {
				h.playbackError(w, r, playback.ErrUnavailable)
				return
			}
			lines := strings.Split(string(body), "\n")
			for i, line := range lines {
				trimmed := strings.TrimSpace(line)
				if trimmed != "" && !strings.HasPrefix(trimmed, "#") {
					lines[i] = appendMediaToken(line, token)
				} else if strings.Contains(trimmed, `URI="`) {
					lines[i] = appendHLSAttributeToken(line, token)
				}
			}
			w.Header().Set("Cache-Control", "private, no-store")
			w.Header().Set("X-Content-Type-Options", "nosniff")
			_, _ = w.Write([]byte(strings.Join(lines, "\n")))
			return
		}
	} else if strings.HasSuffix(p, ".m4s") {
		w.Header().Set("Content-Type", "video/iso.segment")
	} else {
		w.Header().Set("Content-Type", "video/mp4")
	}
	w.Header().Set("Cache-Control", "private, no-store")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	http.ServeFile(w, r, p)
}

func appendMediaToken(value, token string) string {
	separator := "?"
	if strings.Contains(value, "?") {
		separator = "&"
	}
	return value + separator + "token=" + url.QueryEscape(token)
}

func appendHLSAttributeToken(line, token string) string {
	start := strings.Index(line, `URI="`)
	if start < 0 {
		return line
	}
	start += len(`URI="`)
	end := strings.Index(line[start:], `"`)
	if end < 0 {
		return line
	}
	end += start
	return line[:start] + appendMediaToken(line[start:end], token) + line[end:]
}

func (h *Handler) playbackMedia(w http.ResponseWriter, r *http.Request) {
	token := mediaAccessToken(r)
	ownerVerified := false
	if bearer(r) != "" {
		p, e := h.auth.Authenticate(bearer(r))
		if e != nil || !h.playback.OwnedBy(r.Context(), r.PathValue("sessionId"), p.UserID, p.SessionID) {
			h.playbackError(w, r, playback.ErrForbidden)
			return
		}
		ownerVerified = true
	}
	var a playback.MediaAccess
	var err error
	if ownerVerified {
		a, err = h.playback.AuthorizeMediaForOwner(r.Context(), r.PathValue("sessionId"))
	} else {
		a, err = h.playback.AuthorizeMedia(r.Context(), r.PathValue("sessionId"), token)
	}
	if err != nil {
		h.playbackError(w, r, err)
		return
	}
	if a.Mode != playback.DirectPlay {
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
	token := mediaAccessToken(r)
	var b []byte
	var e error
	if raw := bearer(r); raw != "" {
		principal, authErr := h.auth.Authenticate(raw)
		if authErr != nil || !h.playback.OwnedBy(r.Context(), r.PathValue("sessionId"), principal.UserID, principal.SessionID) {
			h.playbackError(w, r, playback.ErrForbidden)
			return
		}
		b, e = h.playback.SubtitleForOwner(r.Context(), r.PathValue("sessionId"), r.PathValue("trackId"))
	} else {
		b, e = h.playback.Subtitle(r.Context(), r.PathValue("sessionId"), token, r.PathValue("trackId"))
	}
	if e != nil {
		h.playbackError(w, r, e)
		return
	}
	w.Header().Set("Content-Type", "text/vtt; charset=utf-8")
	w.Header().Set("Cache-Control", "private, no-store")
	w.WriteHeader(200)
	_, _ = w.Write(b)
}
func mediaAccessToken(r *http.Request) string {
	if c, err := r.Cookie(mediaCookie(r.PathValue("sessionId"))); err == nil && strings.TrimSpace(c.Value) != "" {
		return c.Value
	}
	return strings.TrimSpace(r.URL.Query().Get("token"))
}
func (h *Handler) continueWatching(w http.ResponseWriter, r *http.Request, p auth.Principal) {
	v, e := h.playback.ContinueWatching(r.Context(), p.UserID)
	if e != nil {
		h.playbackError(w, r, e)
		return
	}
	if p.Role == auth.RoleUser {
		out := v[:0]
		for _, item := range v {
			if h.sharing.HasLogical(r.Context(), p, item.LogicalType, item.LogicalID, "VIEW") {
				out = append(out, item)
			}
		}
		v = out
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
