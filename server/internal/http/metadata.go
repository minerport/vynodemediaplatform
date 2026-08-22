package httpserver

import (
	"errors"
	"net/http"
	"os"
	"strconv"

	"github.com/vynode/media/server/internal/auth"
	"github.com/vynode/media/server/internal/metadata"
)

func (h *Handler) metadataRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/v1/movies", h.require(auth.CapLogicalMediaView, h.movies))
	mux.HandleFunc("GET /api/v1/movies/{id}", h.require(auth.CapLogicalMediaView, h.movie))
	mux.HandleFunc("GET /api/v1/shows", h.require(auth.CapLogicalMediaView, h.shows))
	mux.HandleFunc("GET /api/v1/shows/{id}", h.require(auth.CapLogicalMediaView, h.show))
	mux.HandleFunc("GET /api/v1/search", h.require(auth.CapLogicalMediaView, h.localSearch))
	mux.HandleFunc("GET /api/v1/admin/metadata/provider", h.require(auth.CapProviderManage, h.providerStatus))
	mux.HandleFunc("PUT /api/v1/admin/metadata/provider", h.require(auth.CapProviderManage, h.configureProvider))
	mux.HandleFunc("POST /api/v1/admin/metadata/provider/test", h.require(auth.CapProviderManage, h.testProvider))
	mux.HandleFunc("GET /api/v1/admin/metadata/provider/search", h.require(auth.CapMetadataManage, h.providerSearch))
	mux.HandleFunc("GET /api/v1/admin/metadata/unmatched", h.require(auth.CapMetadataManage, h.unmatched))
	mux.HandleFunc("POST /api/v1/admin/metadata/files/{fileId}/match", h.require(auth.CapMetadataManage, h.manualMatch))
	mux.HandleFunc("POST /api/v1/admin/metadata/files/{fileId}/unmatch", h.require(auth.CapMetadataManage, h.unmatch))
	mux.HandleFunc("POST /api/v1/libraries/{libraryId}/identify", h.require(auth.CapMetadataManage, h.identify))
	mux.HandleFunc("GET /api/v1/libraries/{libraryId}/metadata-jobs/{jobId}", h.require(auth.CapMetadataManage, h.metadataJob))
	mux.HandleFunc("GET /api/v1/movies/{id}/artwork", h.require(auth.CapLogicalMediaView, h.movieArtwork))
	mux.HandleFunc("POST /api/v1/movies/{id}/artwork/{artworkId}/select", h.require(auth.CapMetadataManage, h.selectMovieArtwork))
	mux.HandleFunc("GET /api/v1/shows/{id}/artwork", h.require(auth.CapLogicalMediaView, h.showArtwork))
	mux.HandleFunc("POST /api/v1/shows/{id}/artwork/{artworkId}/select", h.require(auth.CapMetadataManage, h.selectShowArtwork))
	mux.HandleFunc("GET /api/v1/artwork/{artworkId}/content", h.require(auth.CapLogicalMediaView, h.artworkContent))
	mux.HandleFunc("POST /api/v1/movies/{id}/refresh", h.require(auth.CapMetadataManage, h.refreshMovie))
	mux.HandleFunc("POST /api/v1/shows/{id}/refresh", h.require(auth.CapMetadataManage, h.refreshShow))
}
func page(r *http.Request) (int, int) {
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	offset, _ := strconv.Atoi(r.URL.Query().Get("offset"))
	return limit, offset
}
func (h *Handler) movies(w http.ResponseWriter, r *http.Request, _ auth.Principal) {
	l, o := page(r)
	v, e := h.metadata.Movies(r.Context(), r.URL.Query().Get("q"), l, o)
	if e != nil {
		h.metadataError(w, r, e)
		return
	}
	writeJSON(w, 200, map[string]any{"movies": v, "limit": l, "offset": o})
}
func (h *Handler) movie(w http.ResponseWriter, r *http.Request, _ auth.Principal) {
	v, e := h.metadata.Movie(r.Context(), r.PathValue("id"))
	if e != nil {
		h.metadataError(w, r, e)
		return
	}
	writeJSON(w, 200, v)
}
func (h *Handler) shows(w http.ResponseWriter, r *http.Request, _ auth.Principal) {
	l, o := page(r)
	v, e := h.metadata.Shows(r.Context(), r.URL.Query().Get("q"), l, o)
	if e != nil {
		h.metadataError(w, r, e)
		return
	}
	writeJSON(w, 200, map[string]any{"shows": v, "limit": l, "offset": o})
}
func (h *Handler) show(w http.ResponseWriter, r *http.Request, _ auth.Principal) {
	v, e := h.metadata.Show(r.Context(), r.PathValue("id"))
	if e != nil {
		h.metadataError(w, r, e)
		return
	}
	writeJSON(w, 200, v)
}
func (h *Handler) localSearch(w http.ResponseWriter, r *http.Request, _ auth.Principal) {
	q := r.URL.Query().Get("q")
	m, e := h.metadata.Movies(r.Context(), q, 25, 0)
	if e != nil {
		h.metadataError(w, r, e)
		return
	}
	s, e := h.metadata.Shows(r.Context(), q, 25, 0)
	if e != nil {
		h.metadataError(w, r, e)
		return
	}
	writeJSON(w, 200, map[string]any{"movies": m, "shows": s})
}
func (h *Handler) providerStatus(w http.ResponseWriter, r *http.Request, _ auth.Principal) {
	writeJSON(w, 200, h.metadata.ProviderStatus(r.Context()))
}
func (h *Handler) configureProvider(w http.ResponseWriter, r *http.Request, p auth.Principal) {
	var in struct {
		Enabled                 bool `json:"enabled"`
		Token, Language, Region string
	}
	if !decode(w, r, &in) {
		return
	}
	if e := h.metadata.Configure(r.Context(), in.Enabled, in.Token, in.Language, in.Region); e != nil {
		h.metadataError(w, r, e)
		return
	}
	event := "METADATA_PROVIDER_CONFIGURED"
	if !in.Enabled {
		event = "METADATA_PROVIDER_DISABLED"
	}
	h.metadata.Audit(r.Context(), p.UserID, event, "TMDB", RequestID(r.Context()))
	writeJSON(w, 200, h.metadata.ProviderStatus(r.Context()))
}
func (h *Handler) testProvider(w http.ResponseWriter, r *http.Request, _ auth.Principal) {
	if e := h.metadata.TestProvider(r.Context()); e != nil {
		h.metadataError(w, r, e)
		return
	}
	writeJSON(w, 200, map[string]string{"status": "available"})
}
func (h *Handler) providerSearch(w http.ResponseWriter, r *http.Request, _ auth.Principal) {
	year, _ := strconv.Atoi(r.URL.Query().Get("year"))
	v, e := h.metadata.SearchProvider(r.Context(), r.URL.Query().Get("type"), r.URL.Query().Get("q"), year)
	if e != nil {
		h.metadataError(w, r, e)
		return
	}
	writeJSON(w, 200, map[string]any{"candidates": v})
}
func (h *Handler) unmatched(w http.ResponseWriter, r *http.Request, _ auth.Principal) {
	v, e := h.metadata.Unmatched(r.Context())
	if e != nil {
		h.metadataError(w, r, e)
		return
	}
	writeJSON(w, 200, map[string]any{"items": v})
}
func (h *Handler) manualMatch(w http.ResponseWriter, r *http.Request, p auth.Principal) {
	var in struct {
		Type, ProviderID                 string
		Season, EpisodeStart, EpisodeEnd int
	}
	if !decode(w, r, &in) {
		return
	}
	var id string
	var e error
	if in.Type == "MOVIE" {
		id, e = h.metadata.MatchMovie(r.Context(), r.PathValue("fileId"), in.ProviderID, true)
	} else if in.Type == "EPISODE" {
		id, e = h.metadata.MatchTV(r.Context(), r.PathValue("fileId"), in.ProviderID, in.Season, in.EpisodeStart, in.EpisodeEnd, true)
	} else {
		e = metadata.ErrValidation
	}
	if e != nil {
		h.metadataError(w, r, e)
		return
	}
	h.metadata.Audit(r.Context(), p.UserID, "MEDIA_MANUALLY_MATCHED", r.PathValue("fileId"), RequestID(r.Context()))
	writeJSON(w, 200, map[string]string{"logicalId": id})
}
func (h *Handler) unmatch(w http.ResponseWriter, r *http.Request, p auth.Principal) {
	if e := h.metadata.Unmatch(r.Context(), r.PathValue("fileId")); e != nil {
		h.metadataError(w, r, e)
		return
	}
	h.metadata.Audit(r.Context(), p.UserID, "MEDIA_UNMATCHED", r.PathValue("fileId"), RequestID(r.Context()))
	w.WriteHeader(204)
}
func (h *Handler) identify(w http.ResponseWriter, r *http.Request, p auth.Principal) {
	j, e := h.metadata.StartIdentify(r.Context(), r.PathValue("libraryId"))
	if e != nil {
		h.metadataError(w, r, e)
		return
	}
	h.metadata.Audit(r.Context(), p.UserID, "METADATA_REFRESH_STARTED", j.ID, RequestID(r.Context()))
	writeJSON(w, 202, j)
}
func (h *Handler) metadataJob(w http.ResponseWriter, r *http.Request, _ auth.Principal) {
	j, e := h.metadata.GetMetadataJob(r.Context(), r.PathValue("libraryId"), r.PathValue("jobId"))
	if e != nil {
		h.metadataError(w, r, e)
		return
	}
	writeJSON(w, 200, j)
}
func (h *Handler) refreshMovie(w http.ResponseWriter, r *http.Request, p auth.Principal) {
	h.refreshLogical(w, r, p, "MOVIE")
}
func (h *Handler) refreshShow(w http.ResponseWriter, r *http.Request, p auth.Principal) {
	h.refreshLogical(w, r, p, "SHOW")
}
func (h *Handler) refreshLogical(w http.ResponseWriter, r *http.Request, p auth.Principal, kind string) {
	if e := h.metadata.Refresh(r.Context(), kind, r.PathValue("id")); e != nil {
		h.metadataError(w, r, e)
		return
	}
	h.metadata.Audit(r.Context(), p.UserID, "METADATA_REFRESH_STARTED", r.PathValue("id"), RequestID(r.Context()))
	w.WriteHeader(204)
}
func(h *Handler)movieArtwork(w http.ResponseWriter,r *http.Request,p auth.Principal){h.artworkList(w,r,p,"MOVIE")}
func(h *Handler)showArtwork(w http.ResponseWriter,r *http.Request,p auth.Principal){h.artworkList(w,r,p,"SHOW")}
func (h *Handler) artworkList(w http.ResponseWriter, r *http.Request, _ auth.Principal, kind string) {
	v, e := h.metadata.Artwork(r.Context(), kind, r.PathValue("id"))
	if e != nil {
		h.metadataError(w, r, e)
		return
	}
	writeJSON(w, 200, map[string]any{"artwork": v})
}
func(h *Handler)selectMovieArtwork(w http.ResponseWriter,r *http.Request,p auth.Principal){h.artworkSelect(w,r,p,"MOVIE")}
func(h *Handler)selectShowArtwork(w http.ResponseWriter,r *http.Request,p auth.Principal){h.artworkSelect(w,r,p,"SHOW")}
func (h *Handler) artworkSelect(w http.ResponseWriter, r *http.Request, p auth.Principal, kind string) {
	if e := h.metadata.SelectArtwork(r.Context(), kind, r.PathValue("id"), r.PathValue("artworkId")); e != nil {
		h.metadataError(w, r, e)
		return
	}
	h.metadata.Audit(r.Context(), p.UserID, "ARTWORK_SELECTION_CHANGED", r.PathValue("artworkId"), RequestID(r.Context()))
	w.WriteHeader(204)
}
func (h *Handler) artworkContent(w http.ResponseWriter, r *http.Request, _ auth.Principal) {
	path, mime, etag, e := h.metadata.ArtworkFile(r.Context(), r.PathValue("artworkId"))
	if e != nil {
		h.metadataError(w, r, e)
		return
	}
	if etag != "" && r.Header.Get("If-None-Match") == etag {
		w.WriteHeader(304)
		return
	}
	f, e := os.Open(path)
	if e != nil {
		h.metadataError(w, r, metadata.ErrNotFound)
		return
	}
	defer f.Close()
	st, e := f.Stat()
	if e != nil {
		h.metadataError(w, r, e)
		return
	}
	w.Header().Set("Content-Type", mime)
	w.Header().Set("Cache-Control", "private, max-age=86400")
	w.Header().Set("ETag", etag)
	http.ServeContent(w, r, st.Name(), st.ModTime(), f)
}
func (h *Handler) metadataError(w http.ResponseWriter, r *http.Request, e error) {
	switch {
	case errors.Is(e, metadata.ErrValidation):
		writeError(w, r, 400, "VALIDATION_FAILED", "The metadata request is invalid.")
	case errors.Is(e, metadata.ErrNotFound):
		writeError(w, r, 404, "NOT_FOUND", "The logical media item was not found.")
	case errors.Is(e, metadata.ErrUnauthorized):
		writeError(w, r, 502, "PROVIDER_UNAUTHORIZED", "The metadata provider credential was rejected.")
	case errors.Is(e, metadata.ErrRateLimited):
		writeError(w, r, 503, "PROVIDER_RATE_LIMITED", "The metadata provider is temporarily rate limited.")
	case errors.Is(e, metadata.ErrProviderUnavailable):
		writeError(w, r, 503, "PROVIDER_UNAVAILABLE", "The metadata provider is unavailable.")
	default:
		writeError(w, r, 500, "INTERNAL_ERROR", "An unexpected error occurred.")
	}
}
