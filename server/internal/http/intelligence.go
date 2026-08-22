package httpserver

import (
	"errors"
	"net/http"
	"strings"

	"github.com/vynode/media/server/internal/auth"
	"github.com/vynode/media/server/internal/intelligence"
)

func (h *Handler) intelligenceRoutes(m *http.ServeMux) {
	m.HandleFunc("GET /api/v1/admin/background-jobs", h.require(auth.CapMetadataManage, h.backgroundJobs))
	m.HandleFunc("DELETE /api/v1/admin/background-jobs/{id}", h.require(auth.CapMetadataManage, h.cancelBackgroundJob))
	m.HandleFunc("POST /api/v1/admin/marker-analysis", h.require(auth.CapMetadataManage, h.startMarkerAnalysis))
	m.HandleFunc("GET /api/v1/admin/marker-review", h.require(auth.CapMetadataManage, h.markerReviewQueue))
	m.HandleFunc("POST /api/v1/admin/marker-review/{id}", h.require(auth.CapMetadataManage, h.reviewMarker))
	m.HandleFunc("GET /api/v1/admin/marker-policy", h.require(auth.CapMetadataManage, h.markerPolicy))
	m.HandleFunc("PUT /api/v1/admin/marker-policy", h.require(auth.CapMetadataManage, h.setMarkerPolicy))
	m.HandleFunc("POST /api/v1/admin/optimizations", h.require(auth.CapMetadataManage, h.createOptimization))
	m.HandleFunc("GET /api/v1/admin/optimizations", h.require(auth.CapMetadataManage, h.optimizations))
	m.HandleFunc("DELETE /api/v1/admin/optimizations/{id}", h.require(auth.CapMetadataManage, h.deleteOptimization))
	m.HandleFunc("GET /api/v1/admin/automation-rules", h.require(auth.CapMetadataManage, h.automationRules))
	m.HandleFunc("POST /api/v1/admin/automation-rules", h.require(auth.CapMetadataManage, h.saveAutomationRule))
	m.HandleFunc("PUT /api/v1/admin/automation-rules/{id}", h.require(auth.CapMetadataManage, h.saveAutomationRule))
	m.HandleFunc("DELETE /api/v1/admin/automation-rules/{id}", h.require(auth.CapMetadataManage, h.deleteAutomationRule))
	m.HandleFunc("POST /api/v1/admin/automation-rules/dry-run", h.require(auth.CapMetadataManage, h.dryRunAutomation))
	m.HandleFunc("POST /api/v1/admin/automation-rules/{id}/execute", h.require(auth.CapMetadataManage, h.executeAutomation))
}
func (h *Handler) intelError(w http.ResponseWriter, r *http.Request, e error) {
	switch {
	case errors.Is(e, intelligence.ErrValidation):
		writeError(w, r, 400, "validation_failed", "The intelligence request is invalid.")
	case errors.Is(e, intelligence.ErrNotFound):
		writeError(w, r, 404, "not_found", "The requested intelligence resource was not found.")
	case errors.Is(e, intelligence.ErrConflict):
		writeError(w, r, 409, "manual_precedence", "A manual marker is authoritative.")
	default:
		writeError(w, r, 500, "intelligence_failed", "The background operation failed.")
	}
}
func (h *Handler) backgroundJobs(w http.ResponseWriter, r *http.Request, _ auth.Principal) {
	x, e := h.intelligence.Jobs(r.Context())
	if e != nil {
		h.intelError(w, r, e)
		return
	}
	writeJSON(w, 200, map[string]any{"jobs": x})
}
func (h *Handler) cancelBackgroundJob(w http.ResponseWriter, r *http.Request, _ auth.Principal) {
	if e := h.intelligence.Cancel(r.Context(), r.PathValue("id")); e != nil {
		h.intelError(w, r, e)
		return
	}
	w.WriteHeader(204)
}
func (h *Handler) startMarkerAnalysis(w http.ResponseWriter, r *http.Request, p auth.Principal) {
	var in struct{ TargetType, TargetID string }
	if !decode(w, r, &in) {
		return
	}
	x, e := h.intelligence.Analyze(r.Context(), strings.ToUpper(in.TargetType), in.TargetID)
	if e != nil {
		h.intelError(w, r, e)
		return
	}
	_ = h.auth.Audit(r.Context(), "MARKER_ANALYSIS_REQUESTED", &p.UserID, "background_job", x.ID, RequestID(r.Context()), map[string]any{"targetType": in.TargetType, "targetId": in.TargetID})
	writeJSON(w, 202, x)
}
func (h *Handler) markerReviewQueue(w http.ResponseWriter, r *http.Request, _ auth.Principal) {
	x, e := h.intelligence.ReviewQueue(r.Context())
	if e != nil {
		h.intelError(w, r, e)
		return
	}
	writeJSON(w, 200, map[string]any{"candidates": x})
}
func (h *Handler) reviewMarker(w http.ResponseWriter, r *http.Request, p auth.Principal) {
	var in struct {
		Action     string
		Start, End *float64
	}
	if !decode(w, r, &in) {
		return
	}
	e := h.intelligence.Review(r.Context(), r.PathValue("id"), strings.ToUpper(in.Action), in.Start, in.End)
	if e != nil {
		h.intelError(w, r, e)
		return
	}
	_ = h.auth.Audit(r.Context(), "AUTOMATIC_MARKER_REVIEWED", &p.UserID, "media_marker", r.PathValue("id"), RequestID(r.Context()), map[string]any{"action": in.Action})
	w.WriteHeader(204)
}
func (h *Handler) markerPolicy(w http.ResponseWriter, r *http.Request, _ auth.Principal) {
	x, e := h.intelligence.Policy(r.Context())
	if e != nil {
		h.intelError(w, r, e)
		return
	}
	writeJSON(w, 200, map[string]bool{"automaticallyActivateHighConfidence": x})
}
func (h *Handler) setMarkerPolicy(w http.ResponseWriter, r *http.Request, p auth.Principal) {
	var in struct{ AutomaticallyActivateHighConfidence bool }
	if !decode(w, r, &in) {
		return
	}
	if e := h.intelligence.SetPolicy(r.Context(), in.AutomaticallyActivateHighConfidence); e != nil {
		h.intelError(w, r, e)
		return
	}
	_ = h.auth.Audit(r.Context(), "MARKER_POLICY_UPDATED", &p.UserID, "server_setting", "automatic_high_confidence_markers", RequestID(r.Context()), map[string]any{"enabled": in.AutomaticallyActivateHighConfidence})
	w.WriteHeader(204)
}
func (h *Handler) createOptimization(w http.ResponseWriter, r *http.Request, p auth.Principal) {
	var in struct{ LogicalType, LogicalID, SourceMediaFileID, Profile string }
	if !decode(w, r, &in) {
		return
	}
	x, e := h.intelligence.Optimize(r.Context(), strings.ToUpper(in.LogicalType), in.LogicalID, in.SourceMediaFileID, in.Profile)
	if e != nil {
		h.intelError(w, r, e)
		return
	}
	_ = h.auth.Audit(r.Context(), "MEDIA_OPTIMIZATION_REQUESTED", &p.UserID, "background_job", x.ID, RequestID(r.Context()), map[string]any{"profile": in.Profile})
	writeJSON(w, 202, x)
}
func (h *Handler) optimizations(w http.ResponseWriter, r *http.Request, _ auth.Principal) {
	x, e := h.intelligence.Optimized(r.Context())
	if e != nil {
		h.intelError(w, r, e)
		return
	}
	writeJSON(w, 200, map[string]any{"items": x})
}
func (h *Handler) deleteOptimization(w http.ResponseWriter, r *http.Request, p auth.Principal) {
	if e := h.intelligence.DeleteOptimized(r.Context(), r.PathValue("id")); e != nil {
		h.intelError(w, r, e)
		return
	}
	_ = h.auth.Audit(r.Context(), "OPTIMIZED_MEDIA_DELETED", &p.UserID, "optimized_media", r.PathValue("id"), RequestID(r.Context()), map[string]any{})
	w.WriteHeader(204)
}
func (h *Handler) automationRules(w http.ResponseWriter, r *http.Request, _ auth.Principal) {
	x, e := h.intelligence.Rules(r.Context())
	if e != nil {
		h.intelError(w, r, e)
		return
	}
	writeJSON(w, 200, map[string]any{"rules": x})
}
func (h *Handler) saveAutomationRule(w http.ResponseWriter, r *http.Request, p auth.Principal) {
	var in intelligence.Rule
	if !decode(w, r, &in) {
		return
	}
	if id := r.PathValue("id"); id != "" {
		in.ID = id
	}
	x, e := h.intelligence.SaveRule(r.Context(), in)
	if e != nil {
		h.intelError(w, r, e)
		return
	}
	_ = h.auth.Audit(r.Context(), "AUTOMATION_RULE_SAVED", &p.UserID, "automation_rule", x.ID, RequestID(r.Context()), map[string]any{"name": x.Name})
	writeJSON(w, 200, x)
}
func (h *Handler) deleteAutomationRule(w http.ResponseWriter, r *http.Request, p auth.Principal) {
	if e := h.intelligence.DeleteRule(r.Context(), r.PathValue("id")); e != nil {
		h.intelError(w, r, e)
		return
	}
	_ = h.auth.Audit(r.Context(), "AUTOMATION_RULE_DELETED", &p.UserID, "automation_rule", r.PathValue("id"), RequestID(r.Context()), map[string]any{})
	w.WriteHeader(204)
}
func (h *Handler) dryRunAutomation(w http.ResponseWriter, r *http.Request, _ auth.Principal) {
	var in intelligence.Rule
	if !decode(w, r, &in) {
		return
	}
	x, e := h.intelligence.DryRun(r.Context(), in)
	if e != nil {
		h.intelError(w, r, e)
		return
	}
	writeJSON(w, 200, x)
}
func (h *Handler) executeAutomation(w http.ResponseWriter, r *http.Request, _ auth.Principal) {
	x, e := h.intelligence.Execute(r.Context(), r.PathValue("id"), "interactive:"+RequestID(r.Context()), 0)
	if e != nil {
		h.intelError(w, r, e)
		return
	}
	writeJSON(w, 200, x)
}
