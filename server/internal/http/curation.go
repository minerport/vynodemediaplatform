package httpserver

import (
	"errors"
	"net/http"
	"strings"

	"github.com/vynode/media/server/internal/auth"
	"github.com/vynode/media/server/internal/curation"
)

func (h *Handler) curationRoutes(m *http.ServeMux) {
	view := auth.CapLogicalMediaView
	self := auth.CapCurationSelfManage
	m.HandleFunc("GET /api/v1/collections", h.require(view, h.collections))
	m.HandleFunc("POST /api/v1/collections", h.require(self, h.saveCollection))
	m.HandleFunc("GET /api/v1/collections/{id}", h.require(view, h.collection))
	m.HandleFunc("PATCH /api/v1/collections/{id}", h.require(self, h.saveCollection))
	m.HandleFunc("DELETE /api/v1/collections/{id}", h.require(self, h.deleteCollection))
	m.HandleFunc("POST /api/v1/collections/{id}/items", h.require(self, h.addCollectionItems))
	m.HandleFunc("DELETE /api/v1/collections/{id}/items/{type}/{itemId}", h.require(self, h.removeCollectionItem))
	m.HandleFunc("PUT /api/v1/collections/{id}/order", h.require(self, h.reorderCollection))
	m.HandleFunc("GET /api/v1/smart-collections", h.require(view, h.smarts))
	m.HandleFunc("POST /api/v1/smart-collections", h.require(self, h.saveSmart))
	m.HandleFunc("POST /api/v1/smart-collections/preview", h.require(self, h.previewSmart))
	m.HandleFunc("GET /api/v1/smart-collections/{id}", h.require(view, h.smart))
	m.HandleFunc("PATCH /api/v1/smart-collections/{id}", h.require(self, h.saveSmart))
	m.HandleFunc("DELETE /api/v1/smart-collections/{id}", h.require(self, h.deleteSmart))
	m.HandleFunc("GET /api/v1/playlists", h.require(self, h.playlists))
	m.HandleFunc("POST /api/v1/playlists", h.require(self, h.savePlaylist))
	m.HandleFunc("GET /api/v1/playlists/{id}", h.require(self, h.playlist))
	m.HandleFunc("PATCH /api/v1/playlists/{id}", h.require(self, h.savePlaylist))
	m.HandleFunc("DELETE /api/v1/playlists/{id}", h.require(self, h.deletePlaylist))
	m.HandleFunc("POST /api/v1/playlists/{id}/items", h.require(self, h.addPlaylistItem))
	m.HandleFunc("DELETE /api/v1/playlists/{id}/items/{entryId}", h.require(self, h.removePlaylistItem))
	m.HandleFunc("PUT /api/v1/playlists/{id}/order", h.require(self, h.reorderPlaylist))
	m.HandleFunc("GET /api/v1/playlists/{id}/navigation/{entryId}", h.require(self, h.playlistNavigation))
	for _, base := range []string{"watchlist", "favorites"} {
		m.HandleFunc("GET /api/v1/"+base, h.require(self, h.personal))
		m.HandleFunc("PUT /api/v1/"+base+"/{type}/{itemId}", h.require(self, h.togglePersonal))
		m.HandleFunc("DELETE /api/v1/"+base+"/{type}/{itemId}", h.require(self, h.togglePersonal))
	}
	m.HandleFunc("GET /api/v1/home", h.require(view, h.home))
	m.HandleFunc("GET /api/v1/account/home-rows", h.require(self, h.homeRows))
	m.HandleFunc("POST /api/v1/account/home-rows", h.require(self, h.saveHomeRow))
	m.HandleFunc("PATCH /api/v1/account/home-rows/{id}", h.require(self, h.saveHomeRow))
	m.HandleFunc("DELETE /api/v1/account/home-rows/{id}", h.require(self, h.deleteHomeRow))
	m.HandleFunc("PUT /api/v1/account/home-rows/order", h.require(self, h.reorderHome))
}
func admin(p auth.Principal) bool { return p.Role == auth.RoleOwner || p.Role == auth.RoleAdmin }
func (h *Handler) accessibleItems(r *http.Request, p auth.Principal, items []curation.Item) []curation.Item {
	if p.Role != auth.RoleUser {
		return items
	}
	out := items[:0]
	for _, x := range items {
		if h.sharing.HasLogical(r.Context(), p, x.Type, x.ID, "VIEW") {
			out = append(out, x)
		}
	}
	return out
}
func (h *Handler) curationError(w http.ResponseWriter, r *http.Request, e error) {
	switch {
	case errors.Is(e, curation.ErrValidation):
		writeError(w, r, 400, "validation_failed", "The curation request is invalid.")
	case errors.Is(e, curation.ErrForbidden):
		writeError(w, r, 403, "forbidden", "This curation resource cannot be modified.")
	case errors.Is(e, curation.ErrNotFound):
		writeError(w, r, 404, "not_found", "The curation resource was not found.")
	default:
		writeError(w, r, 500, "curation_failed", "The curation operation failed.")
	}
}
func (h *Handler) collections(w http.ResponseWriter, r *http.Request, p auth.Principal) {
	x, e := h.curation.Collections(r.Context(), p.UserID)
	if e != nil {
		h.curationError(w, r, e)
		return
	}
	if p.Role == auth.RoleUser {
		out := x[:0]
		for _, c := range x {
			full, e := h.curation.Collection(r.Context(), p.UserID, c.ID)
			if e == nil && len(h.accessibleItems(r, p, full.Items)) > 0 {
				out = append(out, c)
			}
		}
		x = out
	}
	writeJSON(w, 200, map[string]any{"collections": x})
}
func (h *Handler) collection(w http.ResponseWriter, r *http.Request, p auth.Principal) {
	x, e := h.curation.Collection(r.Context(), p.UserID, r.PathValue("id"))
	if e != nil {
		h.curationError(w, r, e)
		return
	}
	x.Items = h.accessibleItems(r, p, x.Items)
	if p.Role == auth.RoleUser && len(x.Items) == 0 {
		h.curationError(w, r, curation.ErrNotFound)
		return
	}
	writeJSON(w, 200, x)
}
func (h *Handler) saveCollection(w http.ResponseWriter, r *http.Request, p auth.Principal) {
	var x curation.Collection
	if !decode(w, r, &x) {
		return
	}
	if id := r.PathValue("id"); id != "" {
		x.ID = id
	}
	x, e := h.curation.SaveCollection(r.Context(), p.UserID, admin(p), x)
	if e != nil {
		h.curationError(w, r, e)
		return
	}
	_ = h.auth.Audit(r.Context(), map[bool]string{true: "COLLECTION_UPDATED", false: "COLLECTION_CREATED"}[r.PathValue("id") != ""], &p.UserID, "collection", x.ID, RequestID(r.Context()), map[string]any{"scope": x.Scope})
	writeJSON(w, map[bool]int{true: 200, false: 201}[r.PathValue("id") != ""], x)
}
func (h *Handler) deleteCollection(w http.ResponseWriter, r *http.Request, p auth.Principal) {
	if e := h.curation.DeleteCollection(r.Context(), p.UserID, admin(p), r.PathValue("id")); e != nil {
		h.curationError(w, r, e)
		return
	}
	_ = h.auth.Audit(r.Context(), "COLLECTION_DELETED", &p.UserID, "collection", r.PathValue("id"), RequestID(r.Context()), map[string]any{})
	w.WriteHeader(204)
}
func (h *Handler) addCollectionItems(w http.ResponseWriter, r *http.Request, p auth.Principal) {
	var x struct{ Items []curation.Item }
	if !decode(w, r, &x) {
		return
	}
	for _, item := range x.Items {
		if !h.sharing.HasLogical(r.Context(), p, item.Type, item.ID, "VIEW") {
			h.curationError(w, r, curation.ErrNotFound)
			return
		}
	}
	if e := h.curation.AddCollectionItems(r.Context(), p.UserID, admin(p), r.PathValue("id"), x.Items); e != nil {
		h.curationError(w, r, e)
		return
	}
	w.WriteHeader(204)
}
func (h *Handler) removeCollectionItem(w http.ResponseWriter, r *http.Request, p auth.Principal) {
	if e := h.curation.RemoveCollectionItem(r.Context(), p.UserID, admin(p), r.PathValue("id"), strings.ToUpper(r.PathValue("type")), r.PathValue("itemId")); e != nil {
		h.curationError(w, r, e)
		return
	}
	w.WriteHeader(204)
}
func (h *Handler) reorderCollection(w http.ResponseWriter, r *http.Request, p auth.Principal) {
	var x struct{ IDs []string }
	if !decode(w, r, &x) {
		return
	}
	if e := h.curation.ReorderCollection(r.Context(), p.UserID, admin(p), r.PathValue("id"), x.IDs); e != nil {
		h.curationError(w, r, e)
		return
	}
	w.WriteHeader(204)
}
func (h *Handler) smarts(w http.ResponseWriter, r *http.Request, p auth.Principal) {
	x, e := h.curation.Smarts(r.Context(), p.UserID)
	if e != nil {
		h.curationError(w, r, e)
		return
	}
	if p.Role == auth.RoleUser {
		out := x[:0]
		for _, c := range x {
			full, e := h.curation.Smart(r.Context(), p.UserID, c.ID)
			if e == nil && len(h.accessibleItems(r, p, full.Items)) > 0 {
				out = append(out, c)
			}
		}
		x = out
	}
	writeJSON(w, 200, map[string]any{"smartCollections": x})
}
func (h *Handler) smart(w http.ResponseWriter, r *http.Request, p auth.Principal) {
	x, e := h.curation.Smart(r.Context(), p.UserID, r.PathValue("id"))
	if e != nil {
		h.curationError(w, r, e)
		return
	}
	x.Items = h.accessibleItems(r, p, x.Items)
	if p.Role == auth.RoleUser && len(x.Items) == 0 {
		h.curationError(w, r, curation.ErrNotFound)
		return
	}
	writeJSON(w, 200, x)
}
func (h *Handler) previewSmart(w http.ResponseWriter, r *http.Request, p auth.Principal) {
	var x curation.SmartCollection
	if !decode(w, r, &x) {
		return
	}
	items, e := h.curation.PreviewSmart(r.Context(), p.UserID, x)
	if e != nil {
		h.curationError(w, r, e)
		return
	}
	items = h.accessibleItems(r, p, items)
	writeJSON(w, 200, map[string]any{"count": len(items), "items": items})
}
func (h *Handler) saveSmart(w http.ResponseWriter, r *http.Request, p auth.Principal) {
	var x curation.SmartCollection
	if !decode(w, r, &x) {
		return
	}
	if v := r.PathValue("id"); v != "" {
		x.ID = v
	}
	x, e := h.curation.SaveSmart(r.Context(), p.UserID, admin(p), x)
	if e != nil {
		h.curationError(w, r, e)
		return
	}
	_ = h.auth.Audit(r.Context(), "SMART_COLLECTION_UPDATED", &p.UserID, "smart_collection", x.ID, RequestID(r.Context()), map[string]any{"scope": x.Scope})
	writeJSON(w, 200, x)
}
func (h *Handler) deleteSmart(w http.ResponseWriter, r *http.Request, p auth.Principal) {
	if e := h.curation.DeleteSmart(r.Context(), p.UserID, admin(p), r.PathValue("id")); e != nil {
		h.curationError(w, r, e)
		return
	}
	_ = h.auth.Audit(r.Context(), "SMART_COLLECTION_DELETED", &p.UserID, "smart_collection", r.PathValue("id"), RequestID(r.Context()), map[string]any{})
	w.WriteHeader(204)
}
func (h *Handler) playlists(w http.ResponseWriter, r *http.Request, p auth.Principal) {
	x, e := h.curation.Playlists(r.Context(), p.UserID)
	if e != nil {
		h.curationError(w, r, e)
		return
	}
	writeJSON(w, 200, map[string]any{"playlists": x})
}
func (h *Handler) playlist(w http.ResponseWriter, r *http.Request, p auth.Principal) {
	x, e := h.curation.Playlist(r.Context(), p.UserID, r.PathValue("id"))
	if e != nil {
		h.curationError(w, r, e)
		return
	}
	x.Items = h.accessibleItems(r, p, x.Items)
	writeJSON(w, 200, x)
}
func (h *Handler) savePlaylist(w http.ResponseWriter, r *http.Request, p auth.Principal) {
	var x curation.Playlist
	if !decode(w, r, &x) {
		return
	}
	if v := r.PathValue("id"); v != "" {
		x.ID = v
	}
	x, e := h.curation.SavePlaylist(r.Context(), p.UserID, x)
	if e != nil {
		h.curationError(w, r, e)
		return
	}
	writeJSON(w, 200, x)
}
func (h *Handler) deletePlaylist(w http.ResponseWriter, r *http.Request, p auth.Principal) {
	if e := h.curation.DeletePlaylist(r.Context(), p.UserID, r.PathValue("id")); e != nil {
		h.curationError(w, r, e)
		return
	}
	w.WriteHeader(204)
}
func (h *Handler) addPlaylistItem(w http.ResponseWriter, r *http.Request, p auth.Principal) {
	var x struct{ Type, ID string }
	if !decode(w, r, &x) {
		return
	}
	if !h.sharing.HasLogical(r.Context(), p, strings.ToUpper(x.Type), x.ID, "VIEW") {
		h.curationError(w, r, curation.ErrNotFound)
		return
	}
	item, e := h.curation.AddPlaylistItem(r.Context(), p.UserID, r.PathValue("id"), strings.ToUpper(x.Type), x.ID)
	if e != nil {
		h.curationError(w, r, e)
		return
	}
	writeJSON(w, 201, item)
}
func (h *Handler) removePlaylistItem(w http.ResponseWriter, r *http.Request, p auth.Principal) {
	if e := h.curation.RemovePlaylistItem(r.Context(), p.UserID, r.PathValue("id"), r.PathValue("entryId")); e != nil {
		h.curationError(w, r, e)
		return
	}
	w.WriteHeader(204)
}
func (h *Handler) reorderPlaylist(w http.ResponseWriter, r *http.Request, p auth.Principal) {
	var x struct{ IDs []string }
	if !decode(w, r, &x) {
		return
	}
	if e := h.curation.ReorderPlaylist(r.Context(), p.UserID, r.PathValue("id"), x.IDs); e != nil {
		h.curationError(w, r, e)
		return
	}
	w.WriteHeader(204)
}
func (h *Handler) playlistNavigation(w http.ResponseWriter, r *http.Request, p auth.Principal) {
	prev, next, e := h.curation.PlaylistNavigation(r.Context(), p.UserID, r.PathValue("id"), r.PathValue("entryId"))
	if e != nil {
		h.curationError(w, r, e)
		return
	}
	writeJSON(w, 200, map[string]any{"previous": prev, "next": next})
}
func (h *Handler) personal(w http.ResponseWriter, r *http.Request, p auth.Principal) {
	kind := "WATCHLIST"
	if strings.Contains(r.URL.Path, "favorites") {
		kind = "FAVORITE"
	}
	x, e := h.curation.Personal(r.Context(), p.UserID, kind, 500)
	if e != nil {
		h.curationError(w, r, e)
		return
	}
	x = h.accessibleItems(r, p, x)
	writeJSON(w, 200, map[string]any{"items": x})
}
func (h *Handler) togglePersonal(w http.ResponseWriter, r *http.Request, p auth.Principal) {
	kind := "WATCHLIST"
	if strings.Contains(r.URL.Path, "favorites") {
		kind = "FAVORITE"
	}
	itemType, itemID := strings.ToUpper(r.PathValue("type")), r.PathValue("itemId")
	if r.Method == "PUT" && !h.sharing.HasLogical(r.Context(), p, itemType, itemID, "VIEW") {
		h.curationError(w, r, curation.ErrNotFound)
		return
	}
	e := h.curation.TogglePersonal(r.Context(), p.UserID, kind, itemType, itemID, r.Method == "PUT")
	if e != nil {
		h.curationError(w, r, e)
		return
	}
	w.WriteHeader(204)
}
func (h *Handler) home(w http.ResponseWriter, r *http.Request, p auth.Principal) {
	x, e := h.curation.Home(r.Context(), p.UserID)
	if e != nil {
		h.curationError(w, r, e)
		return
	}
	for i := range x.Rows {
		x.Rows[i].Items = h.accessibleItems(r, p, x.Rows[i].Items)
	}
	writeJSON(w, 200, x)
}
func (h *Handler) homeRows(w http.ResponseWriter, r *http.Request, p auth.Principal) {
	x, e := h.curation.HomeRows(r.Context(), p.UserID)
	if e != nil {
		h.curationError(w, r, e)
		return
	}
	writeJSON(w, 200, map[string]any{"rows": x})
}
func (h *Handler) saveHomeRow(w http.ResponseWriter, r *http.Request, p auth.Principal) {
	var x curation.HomeRow
	if !decode(w, r, &x) {
		return
	}
	if v := r.PathValue("id"); v != "" {
		x.ID = v
	}
	x, e := h.curation.SaveHomeRow(r.Context(), p.UserID, x)
	if e != nil {
		h.curationError(w, r, e)
		return
	}
	writeJSON(w, 200, x)
}
func (h *Handler) deleteHomeRow(w http.ResponseWriter, r *http.Request, p auth.Principal) {
	if e := h.curation.DeleteHomeRow(r.Context(), p.UserID, r.PathValue("id")); e != nil {
		h.curationError(w, r, e)
		return
	}
	w.WriteHeader(204)
}
func (h *Handler) reorderHome(w http.ResponseWriter, r *http.Request, p auth.Principal) {
	var x struct{ IDs []string }
	if !decode(w, r, &x) {
		return
	}
	if e := h.curation.ReorderHome(r.Context(), p.UserID, x.IDs); e != nil {
		h.curationError(w, r, e)
		return
	}
	w.WriteHeader(204)
}
