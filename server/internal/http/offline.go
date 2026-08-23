package httpserver

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"strconv"

	"github.com/vynode/media/server/internal/auth"
	"github.com/vynode/media/server/internal/offline"
)

func (h *Handler) offlineRoutes(m *http.ServeMux) {
	m.HandleFunc("GET /api/v1/admin/download-settings", h.require(auth.CapObservabilityView, h.downloadSettings))
	m.HandleFunc("PUT /api/v1/admin/download-settings", h.require(auth.CapObservabilityManage, h.saveDownloadSettings))
	m.HandleFunc("GET /api/v1/download-profiles", h.require(auth.CapPlaybackStart, h.downloadProfiles))
	m.HandleFunc("POST /api/v1/downloads/plan", h.require(auth.CapPlaybackStart, h.downloadPlan))
	m.HandleFunc("POST /api/v1/downloads", h.require(auth.CapPlaybackStart, h.createDownload))
	m.HandleFunc("GET /api/v1/downloads", h.require(auth.CapPlaybackStart, h.downloads))
	m.HandleFunc("GET /api/v1/downloads/{id}", h.require(auth.CapPlaybackStart, h.download))
	m.HandleFunc("DELETE /api/v1/downloads/{id}", h.require(auth.CapPlaybackStart, h.removeDownload))
	m.HandleFunc("POST /api/v1/downloads/{id}/cancel", h.require(auth.CapPlaybackStart, h.cancelDownload))
	m.HandleFunc("GET /api/v1/downloads/{id}/manifest", h.require(auth.CapPlaybackStart, h.downloadManifest))
	m.HandleFunc("GET /api/v1/downloads/{id}/file", h.require(auth.CapPlaybackStart, h.downloadFile))
	m.HandleFunc("HEAD /api/v1/downloads/{id}/file", h.require(auth.CapPlaybackStart, h.downloadFile))
	m.HandleFunc("POST /api/v1/sync/push", h.require(auth.CapPlaybackStart, h.syncPush))
	m.HandleFunc("GET /api/v1/sync/changes", h.require(auth.CapPlaybackStart, h.syncPull))
	m.HandleFunc("GET /api/v1/sync/state", h.require(auth.CapPlaybackStart, h.syncState))
	m.HandleFunc("GET /api/v1/download-subscriptions", h.require(auth.CapPlaybackStart, h.subscriptions))
	m.HandleFunc("POST /api/v1/download-subscriptions", h.require(auth.CapPlaybackStart, h.createSubscription))
}
func (h *Handler) downloadSettings(w http.ResponseWriter, r *http.Request, _ auth.Principal) {
	x, e := h.offline.Settings(r.Context())
	if e != nil {
		h.offlineError(w, r, e)
		return
	}
	writeJSON(w, 200, x)
}
func (h *Handler) saveDownloadSettings(w http.ResponseWriter, r *http.Request, _ auth.Principal) {
	var in struct {
		CacheQuotaBytes int64 `json:"cacheQuotaBytes"`
	}
	if !decode(w, r, &in) {
		return
	}
	x, e := h.offline.SetSettings(r.Context(), in.CacheQuotaBytes)
	if e != nil {
		h.offlineError(w, r, e)
		return
	}
	writeJSON(w, 200, x)
}
func (h *Handler) offlineError(w http.ResponseWriter, r *http.Request, e error) {
	switch e {
	case offline.ErrInvalid:
		writeError(w, r, 400, "VALIDATION_FAILED", "The offline request is invalid.")
	case offline.ErrDenied:
		writeError(w, r, 403, "DOWNLOAD_FORBIDDEN", "The paired device or DOWNLOAD grant is not authorized.")
	case offline.ErrNotFound:
		writeError(w, r, 404, "NOT_FOUND", "The offline resource was not found.")
	case offline.ErrNotReady:
		writeError(w, r, 409, "DOWNLOAD_NOT_READY", "The download asset is not ready.")
	case offline.ErrStorage:
		writeError(w, r, 409, "DEVICE_STORAGE_LIMIT", "Storage headroom or cache quota prevents this download.")
	default:
		writeError(w, r, 500, "INTERNAL_ERROR", "An unexpected error occurred.")
	}
}
func (h *Handler) downloadProfiles(w http.ResponseWriter, r *http.Request, _ auth.Principal) {
	x, e := h.offline.Profiles(r.Context())
	if e != nil {
		h.offlineError(w, r, e)
		return
	}
	writeJSON(w, 200, map[string]any{"profiles": x})
}
func (h *Handler) downloadPlan(w http.ResponseWriter, r *http.Request, p auth.Principal) {
	var in offline.CreateRequest
	if !decode(w, r, &in) {
		return
	}
	if !h.sharing.HasLogical(r.Context(), p, in.LogicalType, in.LogicalID, "DOWNLOAD") {
		h.offlineError(w, r, offline.ErrDenied)
		return
	}
	x, e := h.offline.Plan(r.Context(), in.LogicalType, in.LogicalID, in.ProfileID)
	if e != nil {
		h.offlineError(w, r, e)
		return
	}
	writeJSON(w, 200, x)
}
func (h *Handler) createDownload(w http.ResponseWriter, r *http.Request, p auth.Principal) {
	var in offline.CreateRequest
	if !decode(w, r, &in) {
		return
	}
	x, e := h.offline.Create(r.Context(), p, in)
	if e != nil {
		h.offlineError(w, r, e)
		return
	}
	writeJSON(w, 201, x)
}
func (h *Handler) downloads(w http.ResponseWriter, r *http.Request, p auth.Principal) {
	x, e := h.offline.List(r.Context(), p)
	if e != nil {
		h.offlineError(w, r, e)
		return
	}
	writeJSON(w, 200, map[string]any{"downloads": x})
}
func (h *Handler) download(w http.ResponseWriter, r *http.Request, p auth.Principal) {
	x, e := h.offline.Get(r.Context(), p, r.PathValue("id"))
	if e != nil {
		h.offlineError(w, r, e)
		return
	}
	writeJSON(w, 200, x)
}
func (h *Handler) removeDownload(w http.ResponseWriter, r *http.Request, p auth.Principal) {
	if e := h.offline.Delete(r.Context(), p, r.PathValue("id")); e != nil {
		h.offlineError(w, r, e)
		return
	}
	w.WriteHeader(204)
}
func (h *Handler) cancelDownload(w http.ResponseWriter, r *http.Request, p auth.Principal) {
	if e := h.offline.Cancel(r.Context(), p, r.PathValue("id")); e != nil {
		h.offlineError(w, r, e)
		return
	}
	w.WriteHeader(204)
}
func (h *Handler) downloadManifest(w http.ResponseWriter, r *http.Request, p auth.Principal) {
	x, e := h.offline.Manifest(r.Context(), p, r.PathValue("id"))
	if e != nil {
		h.offlineError(w, r, e)
		return
	}
	writeJSON(w, 200, x)
}
func (h *Handler) downloadFile(w http.ResponseWriter, r *http.Request, p auth.Principal) {
	x, path, e := h.offline.Path(r.Context(), p, r.PathValue("id"))
	if e != nil {
		h.offlineError(w, r, e)
		return
	}
	f, e := os.Open(path)
	if e != nil {
		h.offlineError(w, r, offline.ErrNotReady)
		return
	}
	defer f.Close()
	st, e := f.Stat()
	if e != nil {
		h.offlineError(w, r, e)
		return
	}
	manifest, _ := h.offline.Manifest(r.Context(), p, x.ID)
	w.Header().Set("Accept-Ranges", "bytes")
	w.Header().Set("Content-Type", x.ContentType)
	w.Header().Set("ETag", `"sha256-`+x.ChecksumSHA256+`"`)
	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"`, manifest.SuggestedName))
	w.Header().Set("Cache-Control", "private, no-store")
	if r.Header.Get("Range") == "" {
		w.Header().Set("Content-Length", strconv.FormatInt(st.Size(), 10))
		w.WriteHeader(200)
		if r.Method != "HEAD" {
			_, _ = io.Copy(w, f)
		}
		return
	}
	start, end, ok := singleRange(r.Header.Get("Range"), st.Size())
	if !ok {
		w.Header().Set("Content-Range", fmt.Sprintf("bytes */%d", st.Size()))
		w.WriteHeader(416)
		return
	}
	length := end - start + 1
	w.Header().Set("Content-Length", strconv.FormatInt(length, 10))
	w.Header().Set("Content-Range", fmt.Sprintf("bytes %d-%d/%d", start, end, st.Size()))
	w.WriteHeader(206)
	if r.Method != "HEAD" {
		_, _ = io.CopyN(w, io.NewSectionReader(f, start, length), length)
	}
}
func (h *Handler) syncPush(w http.ResponseWriter, r *http.Request, p auth.Principal) {
	var in offline.Push
	if !decode(w, r, &in) {
		return
	}
	x, e := h.offline.Push(r.Context(), p, in)
	if e != nil {
		h.offlineError(w, r, e)
		return
	}
	writeJSON(w, 200, x)
}
func (h *Handler) syncPull(w http.ResponseWriter, r *http.Request, p auth.Principal) {
	cursor, _ := strconv.ParseInt(r.URL.Query().Get("cursor"), 10, 64)
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	x, e := h.offline.Pull(r.Context(), p, cursor, limit)
	if e != nil {
		h.offlineError(w, r, e)
		return
	}
	writeJSON(w, 200, x)
}
func (h *Handler) syncState(w http.ResponseWriter, r *http.Request, p auth.Principal) {
	x, e := h.offline.Pull(r.Context(), p, 0, 100)
	if e != nil {
		h.offlineError(w, r, e)
		return
	}
	x.Downloads, _ = h.offline.List(r.Context(), p)
	x.Subscriptions, _ = h.offline.Subscriptions(r.Context(), p)
	writeJSON(w, 200, x)
}
func (h *Handler) subscriptions(w http.ResponseWriter, r *http.Request, p auth.Principal) {
	x, e := h.offline.Subscriptions(r.Context(), p)
	if e != nil {
		h.offlineError(w, r, e)
		return
	}
	writeJSON(w, 200, map[string]any{"subscriptions": x})
}
func (h *Handler) createSubscription(w http.ResponseWriter, r *http.Request, p auth.Principal) {
	var in offline.SubscriptionRequest
	if !decode(w, r, &in) {
		return
	}
	x, e := h.offline.CreateSubscription(r.Context(), p, in)
	if e != nil {
		h.offlineError(w, r, e)
		return
	}
	writeJSON(w, 201, x)
}
