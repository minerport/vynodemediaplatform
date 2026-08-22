package httpserver

import (
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/vynode/media/server/internal/auth"
	"github.com/vynode/media/server/internal/observability"
)

func (h *Handler) observabilityRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/v1/admin/dashboard", h.require(auth.CapObservabilityView, h.adminDashboard))
	mux.HandleFunc("GET /api/v1/admin/analytics/playback", h.require(auth.CapObservabilityView, h.adminAnalytics))
	mux.HandleFunc("GET /api/v1/account/playback-history/summary", h.require(auth.CapPlaybackSelfManage, h.personalAnalytics))
	mux.HandleFunc("GET /api/v1/admin/library-statistics", h.require(auth.CapObservabilityView, h.libraryStatistics))
	mux.HandleFunc("GET /api/v1/admin/jobs", h.require(auth.CapObservabilityView, h.adminJobs))
	mux.HandleFunc("GET /api/v1/admin/health/issues", h.require(auth.CapObservabilityView, h.healthIssues))
	mux.HandleFunc("POST /api/v1/admin/health/reevaluate", h.require(auth.CapObservabilityManage, h.reevaluateHealth))
	mux.HandleFunc("PUT /api/v1/admin/health/issues/{id}/ignored", h.require(auth.CapObservabilityManage, h.ignoreHealth))
	mux.HandleFunc("DELETE /api/v1/admin/health/issues/{id}/ignored", h.require(auth.CapObservabilityManage, h.unignoreHealth))
	mux.HandleFunc("GET /api/v1/admin/operational-events", h.require(auth.CapObservabilityView, h.operationalEvents))
	mux.HandleFunc("GET /api/v1/admin/notifications/catalog", h.require(auth.CapObservabilityView, h.eventCatalog))
	mux.HandleFunc("GET /api/v1/admin/notifications/destinations", h.require(auth.CapObservabilityView, h.destinations))
	mux.HandleFunc("POST /api/v1/admin/notifications/destinations", h.require(auth.CapObservabilityManage, h.saveDestination))
	mux.HandleFunc("PATCH /api/v1/admin/notifications/destinations/{id}", h.require(auth.CapObservabilityManage, h.updateDestination))
	mux.HandleFunc("DELETE /api/v1/admin/notifications/destinations/{id}", h.require(auth.CapObservabilityManage, h.deleteDestination))
	mux.HandleFunc("POST /api/v1/admin/notifications/destinations/{id}/test", h.require(auth.CapObservabilityManage, h.testDestination))
	mux.HandleFunc("GET /api/v1/admin/notifications/deliveries", h.require(auth.CapObservabilityView, h.deliveries))
}
func (h *Handler) observabilityError(w http.ResponseWriter, r *http.Request, e error) {
	switch {
	case errors.Is(e, observability.ErrValidation):
		writeError(w, r, 400, "VALIDATION_ERROR", "The observability request is invalid or violates destination network policy.")
	case errors.Is(e, observability.ErrNotFound):
		writeError(w, r, 404, "NOT_FOUND", "The requested observability resource was not found.")
	default:
		writeError(w, r, 500, "OBSERVABILITY_ERROR", "The observability operation failed.")
	}
}
func (h *Handler) adminDashboard(w http.ResponseWriter, r *http.Request, _ auth.Principal) {
	x, e := h.observability.Dashboard(r.Context())
	if e != nil {
		h.observabilityError(w, r, e)
		return
	}
	writeJSON(w, 200, x)
}
func analyticsRange(r *http.Request) (time.Time, time.Time, error) {
	to := time.Now().UTC()
	from := to.Add(-7 * 24 * time.Hour)
	var e error
	if v := r.URL.Query().Get("from"); v != "" {
		from, e = time.Parse(time.RFC3339, v)
		if e != nil {
			return from, to, e
		}
	}
	if v := r.URL.Query().Get("to"); v != "" {
		to, e = time.Parse(time.RFC3339, v)
		if e != nil {
			return from, to, e
		}
	}
	if !from.Before(to) || to.Sub(from) > 366*24*time.Hour {
		return from, to, observability.ErrValidation
	}
	return from, to, nil
}
func (h *Handler) adminAnalytics(w http.ResponseWriter, r *http.Request, _ auth.Principal) {
	from, to, e := analyticsRange(r)
	if e != nil {
		h.observabilityError(w, r, e)
		return
	}
	x, e := h.observability.Analytics(r.Context(), from, to, r.URL.Query().Get("userId"))
	if e != nil {
		h.observabilityError(w, r, e)
		return
	}
	writeJSON(w, 200, x)
}
func (h *Handler) personalAnalytics(w http.ResponseWriter, r *http.Request, p auth.Principal) {
	from, to, e := analyticsRange(r)
	if e != nil {
		h.observabilityError(w, r, e)
		return
	}
	x, e := h.observability.Analytics(r.Context(), from, to, p.UserID)
	if e != nil {
		h.observabilityError(w, r, e)
		return
	}
	x.TopUsers = nil
	writeJSON(w, 200, x)
}
func (h *Handler) libraryStatistics(w http.ResponseWriter, r *http.Request, _ auth.Principal) {
	x, e := h.observability.LibraryStats(r.Context())
	if e != nil {
		h.observabilityError(w, r, e)
		return
	}
	writeJSON(w, 200, x)
}
func limit(r *http.Request, d int) int {
	n, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	if n < 1 {
		return d
	}
	return n
}
func (h *Handler) adminJobs(w http.ResponseWriter, r *http.Request, _ auth.Principal) {
	x, e := h.observability.Jobs(r.Context(), limit(r, 100))
	if e != nil {
		h.observabilityError(w, r, e)
		return
	}
	writeJSON(w, 200, map[string]any{"jobs": x})
}
func (h *Handler) healthIssues(w http.ResponseWriter, r *http.Request, _ auth.Principal) {
	x, e := h.observability.Health(r.Context(), strings.ToUpper(r.URL.Query().Get("status")), strings.ToUpper(r.URL.Query().Get("severity")))
	if e != nil {
		h.observabilityError(w, r, e)
		return
	}
	writeJSON(w, 200, map[string]any{"issues": x})
}
func (h *Handler) reevaluateHealth(w http.ResponseWriter, r *http.Request, _ auth.Principal) {
	x, e := h.observability.EvaluateHealth(r.Context())
	if e != nil {
		h.observabilityError(w, r, e)
		return
	}
	writeJSON(w, 200, map[string]any{"issues": x})
}
func (h *Handler) ignoreHealth(w http.ResponseWriter, r *http.Request, p auth.Principal) {
	if e := h.observability.SetHealthIgnored(r.Context(), r.PathValue("id"), true); e != nil {
		h.observabilityError(w, r, e)
		return
	}
	_ = h.auth.Audit(r.Context(), "HEALTH_ISSUE_IGNORED", &p.UserID, "health_issue", r.PathValue("id"), RequestID(r.Context()), map[string]any{})
	w.WriteHeader(204)
}
func (h *Handler) unignoreHealth(w http.ResponseWriter, r *http.Request, p auth.Principal) {
	if e := h.observability.SetHealthIgnored(r.Context(), r.PathValue("id"), false); e != nil {
		h.observabilityError(w, r, e)
		return
	}
	_ = h.auth.Audit(r.Context(), "HEALTH_ISSUE_UNIGNORED", &p.UserID, "health_issue", r.PathValue("id"), RequestID(r.Context()), map[string]any{})
	w.WriteHeader(204)
}
func (h *Handler) operationalEvents(w http.ResponseWriter, r *http.Request, _ auth.Principal) {
	x, e := h.observability.Events(r.Context(), limit(r, 50), 0)
	if e != nil {
		h.observabilityError(w, r, e)
		return
	}
	writeJSON(w, 200, map[string]any{"events": x})
}
func (h *Handler) eventCatalog(w http.ResponseWriter, _ *http.Request, _ auth.Principal) {
	writeJSON(w, 200, map[string]any{"events": observability.EventCatalog})
}
func (h *Handler) destinations(w http.ResponseWriter, r *http.Request, _ auth.Principal) {
	x, e := h.observability.Destinations(r.Context())
	if e != nil {
		h.observabilityError(w, r, e)
		return
	}
	writeJSON(w, 200, map[string]any{"destinations": x})
}
func (h *Handler) saveDestination(w http.ResponseWriter, r *http.Request, p auth.Principal) {
	var x observability.Destination
	if !decode(w, r, &x) {
		return
	}
	x, e := h.observability.SaveDestination(r.Context(), x)
	if e != nil {
		h.observabilityError(w, r, e)
		return
	}
	_ = h.auth.Audit(r.Context(), "WEBHOOK_DESTINATION_CREATED", &p.UserID, "notification_destination", x.ID, RequestID(r.Context()), map[string]any{"allowPrivateNetwork": x.AllowPrivateNetwork, "allowInsecureHTTP": x.AllowInsecureHTTP})
	writeJSON(w, 201, x)
}
func (h *Handler) updateDestination(w http.ResponseWriter, r *http.Request, p auth.Principal) {
	var x observability.Destination
	if !decode(w, r, &x) {
		return
	}
	x.ID = r.PathValue("id")
	x, e := h.observability.SaveDestination(r.Context(), x)
	if e != nil {
		h.observabilityError(w, r, e)
		return
	}
	_ = h.auth.Audit(r.Context(), "WEBHOOK_DESTINATION_UPDATED", &p.UserID, "notification_destination", x.ID, RequestID(r.Context()), map[string]any{"enabled": x.Enabled, "allowPrivateNetwork": x.AllowPrivateNetwork})
	writeJSON(w, 200, x)
}
func (h *Handler) deleteDestination(w http.ResponseWriter, r *http.Request, p auth.Principal) {
	id := r.PathValue("id")
	if e := h.observability.DeleteDestination(r.Context(), id); e != nil {
		h.observabilityError(w, r, e)
		return
	}
	_ = h.auth.Audit(r.Context(), "WEBHOOK_DESTINATION_DELETED", &p.UserID, "notification_destination", id, RequestID(r.Context()), map[string]any{})
	w.WriteHeader(204)
}
func (h *Handler) testDestination(w http.ResponseWriter, r *http.Request, _ auth.Principal) {
	if e := h.observability.TestDestination(r.Context(), r.PathValue("id")); e != nil {
		h.observabilityError(w, r, e)
		return
	}
	w.WriteHeader(202)
}
func (h *Handler) deliveries(w http.ResponseWriter, r *http.Request, _ auth.Principal) {
	x, e := h.observability.Deliveries(r.Context(), limit(r, 50))
	if e != nil {
		h.observabilityError(w, r, e)
		return
	}
	writeJSON(w, 200, map[string]any{"deliveries": x})
}
