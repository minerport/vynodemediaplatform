package httpserver

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/vynode/media/server/internal/auth"
	"github.com/vynode/media/server/internal/media"
)

func (h *Handler) mediaRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/v1/libraries", h.require(auth.CapLibrariesView, h.listLibraries))
	mux.HandleFunc("POST /api/v1/libraries", h.require(auth.CapLibrariesManage, h.createLibrary))
	mux.HandleFunc("GET /api/v1/libraries/{libraryId}", h.require(auth.CapLibrariesView, h.getLibrary))
	mux.HandleFunc("PATCH /api/v1/libraries/{libraryId}", h.require(auth.CapLibrariesManage, h.updateLibrary))
	mux.HandleFunc("DELETE /api/v1/libraries/{libraryId}", h.require(auth.CapLibrariesManage, h.deleteLibrary))
	mux.HandleFunc("POST /api/v1/libraries/sources/validate", h.require(auth.CapLibrariesManage, h.validateSource))
	mux.HandleFunc("POST /api/v1/libraries/{libraryId}/sources", h.require(auth.CapLibrariesManage, h.addSource))
	mux.HandleFunc("DELETE /api/v1/libraries/{libraryId}/sources/{sourceId}", h.require(auth.CapLibrariesManage, h.removeSource))
	mux.HandleFunc("POST /api/v1/libraries/{libraryId}/scan", h.require(auth.CapLibrariesScan, h.startScan))
	mux.HandleFunc("GET /api/v1/libraries/{libraryId}/scans/{jobId}", h.require(auth.CapLibrariesScan, h.getScan))
	mux.HandleFunc("DELETE /api/v1/libraries/{libraryId}/scans/{jobId}", h.require(auth.CapLibrariesScan, h.cancelScan))
	mux.HandleFunc("GET /api/v1/libraries/{libraryId}/items", h.require(auth.CapMediaInventoryView, h.listMediaFiles))
	mux.HandleFunc("GET /api/v1/media/files/{fileId}", h.require(auth.CapMediaInventoryView, h.getMediaFile))
	mux.HandleFunc("GET /api/v1/admin/system/media-probe", h.require(auth.CapSecurityView, h.probeCapability))
}

func (h *Handler) listLibraries(w http.ResponseWriter, r *http.Request, _ auth.Principal) {
	x, e := h.media.ListLibraries(r.Context())
	if e != nil {
		h.mediaError(w, r, e)
		return
	}
	writeJSON(w, 200, map[string]any{"libraries": x})
}
func (h *Handler) createLibrary(w http.ResponseWriter, r *http.Request, p auth.Principal) {
	var in struct {
		Name    string            `json:"name"`
		Type    media.LibraryType `json:"type"`
		Enabled *bool             `json:"enabled"`
		Sources []string          `json:"sources"`
	}
	if !decode(w, r, &in) {
		return
	}
	enabled := true
	if in.Enabled != nil {
		enabled = *in.Enabled
	}
	x, e := h.media.CreateLibrary(r.Context(), in.Name, in.Type, enabled)
	if e != nil {
		h.mediaError(w, r, e)
		return
	}
	for _, path := range in.Sources {
		if _, e = h.media.AddSource(r.Context(), x.ID, path); e != nil {
			_ = h.media.DeleteLibrary(r.Context(), x.ID)
			h.mediaError(w, r, e)
			return
		}
	}
	h.media.Audit(r.Context(), p.UserID, "LIBRARY_CREATED", x.ID, RequestID(r.Context()))
	x, _ = h.media.GetLibrary(r.Context(), x.ID)
	writeJSON(w, 201, x)
}
func (h *Handler) getLibrary(w http.ResponseWriter, r *http.Request, _ auth.Principal) {
	x, e := h.media.GetLibrary(r.Context(), r.PathValue("libraryId"))
	if e != nil {
		h.mediaError(w, r, e)
		return
	}
	writeJSON(w, 200, x)
}
func (h *Handler) updateLibrary(w http.ResponseWriter, r *http.Request, p auth.Principal) {
	var in struct {
		Name    string `json:"name"`
		Enabled *bool  `json:"enabled"`
	}
	if !decode(w, r, &in) {
		return
	}
	x, e := h.media.UpdateLibrary(r.Context(), r.PathValue("libraryId"), in.Name, in.Enabled)
	if e != nil {
		h.mediaError(w, r, e)
		return
	}
	h.media.Audit(r.Context(), p.UserID, "LIBRARY_UPDATED", x.ID, RequestID(r.Context()))
	writeJSON(w, 200, x)
}
func (h *Handler) deleteLibrary(w http.ResponseWriter, r *http.Request, p auth.Principal) {
	id := r.PathValue("libraryId")
	if e := h.media.DeleteLibrary(r.Context(), id); e != nil {
		h.mediaError(w, r, e)
		return
	}
	h.media.Audit(r.Context(), p.UserID, "LIBRARY_REMOVED", id, RequestID(r.Context()))
	w.WriteHeader(204)
}
func (h *Handler) validateSource(w http.ResponseWriter, r *http.Request, _ auth.Principal) {
	var in struct{ Path, LibraryID string }
	if !decode(w, r, &in) {
		return
	}
	normalized, e := h.media.ValidatePath(in.Path, in.LibraryID)
	if e != nil {
		h.mediaError(w, r, e)
		return
	}
	writeJSON(w, 200, map[string]any{"valid": true, "normalizedPath": normalized, "directory": true, "readable": true})
}
func (h *Handler) addSource(w http.ResponseWriter, r *http.Request, p auth.Principal) {
	var in struct {
		Path string `json:"path"`
	}
	if !decode(w, r, &in) {
		return
	}
	x, e := h.media.AddSource(r.Context(), r.PathValue("libraryId"), in.Path)
	if e != nil {
		h.mediaError(w, r, e)
		return
	}
	h.media.Audit(r.Context(), p.UserID, "LIBRARY_SOURCE_ADDED", x.LibraryID, RequestID(r.Context()))
	writeJSON(w, 201, x)
}
func (h *Handler) removeSource(w http.ResponseWriter, r *http.Request, p auth.Principal) {
	id := r.PathValue("libraryId")
	if e := h.media.RemoveSource(r.Context(), id, r.PathValue("sourceId")); e != nil {
		h.mediaError(w, r, e)
		return
	}
	h.media.Audit(r.Context(), p.UserID, "LIBRARY_SOURCE_REMOVED", id, RequestID(r.Context()))
	w.WriteHeader(204)
}
func (h *Handler) startScan(w http.ResponseWriter, r *http.Request, p auth.Principal) {
	j, e := h.media.StartScan(r.Context(), r.PathValue("libraryId"))
	if e != nil {
		if errors.Is(e, media.ErrConflict) {
			writeJSON(w, 409, j)
			return
		}
		h.mediaError(w, r, e)
		return
	}
	h.media.Audit(r.Context(), p.UserID, "LIBRARY_SCAN_STARTED", j.LibraryID, RequestID(r.Context()))
	writeJSON(w, 202, j)
}
func (h *Handler) getScan(w http.ResponseWriter, r *http.Request, _ auth.Principal) {
	j, e := h.media.GetJob(r.Context(), r.PathValue("libraryId"), r.PathValue("jobId"))
	if e != nil {
		h.mediaError(w, r, e)
		return
	}
	writeJSON(w, 200, j)
}
func (h *Handler) cancelScan(w http.ResponseWriter, r *http.Request, p auth.Principal) {
	id := r.PathValue("libraryId")
	if e := h.media.Cancel(id, r.PathValue("jobId")); e != nil {
		h.mediaError(w, r, e)
		return
	}
	h.media.Audit(r.Context(), p.UserID, "LIBRARY_SCAN_CANCELED", id, RequestID(r.Context()))
	w.WriteHeader(204)
}
func (h *Handler) listMediaFiles(w http.ResponseWriter, r *http.Request, _ auth.Principal) {
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	offset, _ := strconv.Atoi(r.URL.Query().Get("offset"))
	x, e := h.media.ListFiles(r.Context(), r.PathValue("libraryId"), limit, offset)
	if e != nil {
		h.mediaError(w, r, e)
		return
	}
	writeJSON(w, 200, map[string]any{"items": x, "limit": limit, "offset": offset})
}
func (h *Handler) getMediaFile(w http.ResponseWriter, r *http.Request, _ auth.Principal) {
	x, e := h.media.GetFile(r.Context(), r.PathValue("fileId"))
	if e != nil {
		h.mediaError(w, r, e)
		return
	}
	writeJSON(w, 200, x)
}
func (h *Handler) probeCapability(w http.ResponseWriter, r *http.Request, _ auth.Principal) {
	writeJSON(w, 200, h.media.Capability(r.Context()))
}
func (h *Handler) mediaError(w http.ResponseWriter, r *http.Request, e error) {
	switch {
	case errors.Is(e, media.ErrValidation):
		writeError(w, r, 400, "INVALID_SOURCE", "The source path is invalid, unavailable, unreadable, or forbidden.")
	case errors.Is(e, media.ErrNotFound):
		writeError(w, r, 404, "NOT_FOUND", "The requested library resource was not found.")
	case errors.Is(e, media.ErrConflict):
		writeError(w, r, 409, "CONFLICT", "The resource conflicts with existing state.")
	case errors.Is(e, media.ErrProbeUnavailable):
		writeError(w, r, 503, "FFPROBE_UNAVAILABLE", "Media inspection is unavailable.")
	default:
		writeError(w, r, 500, "INTERNAL_ERROR", "An unexpected error occurred.")
	}
}
