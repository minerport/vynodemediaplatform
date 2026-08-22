package httpserver

import (
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/vynode/media/server/internal/auth"
	"github.com/vynode/media/server/internal/sharing"
)

func (h *Handler) sharingRoutes(m *http.ServeMux) {
	m.HandleFunc("GET /api/v1/connection-info", h.connectionInfo)
	m.HandleFunc("GET /api/v1/invitations/inspect", h.inspectInvitation)
	m.HandleFunc("POST /api/v1/invitations/accept", h.acceptInvitation)
	m.HandleFunc("GET /api/v1/admin/invitations", h.require(auth.CapUsersManage, h.invitations))
	m.HandleFunc("POST /api/v1/admin/invitations", h.require(auth.CapUsersManage, h.createInvitation))
	m.HandleFunc("DELETE /api/v1/admin/invitations/{id}", h.require(auth.CapUsersManage, h.revokeInvitation))
	m.HandleFunc("GET /api/v1/admin/users/{id}/library-access", h.require(auth.CapUsersManage, h.userGrants))
	m.HandleFunc("PUT /api/v1/admin/users/{id}/library-access", h.require(auth.CapUsersManage, h.setUserGrants))
	m.HandleFunc("POST /api/v1/pairing/requests", h.createPairing)
	m.HandleFunc("GET /api/v1/pairing/requests/{id}", h.pairingStatus)
	m.HandleFunc("POST /api/v1/pairing/requests/{id}/exchange", h.exchangePairing)
	m.HandleFunc("POST /api/v1/account/pairing/lookup", h.protected(h.lookupPairing))
	m.HandleFunc("POST /api/v1/account/pairing/{id}/approve", h.protected(h.approvePairing))
	m.HandleFunc("POST /api/v1/account/pairing/{id}/deny", h.protected(h.denyPairing))
	m.HandleFunc("GET /api/v1/admin/remote-access", h.require(auth.CapServerManage, h.remoteAccess))
	m.HandleFunc("PUT /api/v1/admin/remote-access", h.require(auth.CapServerManage, h.saveRemoteAccess))
}
func (h *Handler) sharingError(w http.ResponseWriter, r *http.Request, e error) {
	switch {
	case errors.Is(e, sharing.ErrInvalid):
		writeError(w, r, 400, "VALIDATION_FAILED", "The request is invalid.")
	case errors.Is(e, sharing.ErrLimited):
		writeError(w, r, 429, "RATE_LIMITED", "Too many attempts. Try again shortly.")
	case errors.Is(e, sharing.ErrDenied):
		writeError(w, r, 403, "FORBIDDEN", "You are not authorized for this operation.")
	case errors.Is(e, sharing.ErrGone):
		writeError(w, r, 410, "INVITATION_OR_PAIRING_UNAVAILABLE", "The invitation or pairing request is expired, used, revoked, or unavailable.")
	default:
		writeError(w, r, 500, "INTERNAL_ERROR", "An unexpected error occurred.")
	}
}
func (h *Handler) connectionInfo(w http.ResponseWriter, r *http.Request) {
	_, secure, _ := h.connection(r)
	writeJSON(w, 200, h.sharing.ConnectionInfo(r.Context(), secure))
}
func (h *Handler) inspectInvitation(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Referrer-Policy", "no-referrer")
	address, _, _ := h.connection(r)
	if !authLimiter.allow("invite-inspect:" + address) {
		h.sharingError(w, r, sharing.ErrLimited)
		return
	}
	x, e := h.sharing.InspectInvite(r.Context(), r.URL.Query().Get("token"))
	if e != nil {
		h.sharingError(w, r, e)
		return
	}
	writeJSON(w, 200, map[string]any{"serverName": h.info.ServerName, "invitation": x})
}
func (h *Handler) acceptInvitation(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Referrer-Policy", "no-referrer")
	address, _, _ := h.connection(r)
	if !authLimiter.allow("invite:" + address) {
		h.sharingError(w, r, sharing.ErrLimited)
		return
	}
	var in struct {
		Token, Username, DisplayName, Password string
		Device                                 auth.DeviceInput
	}
	if !decode(w, r, &in) {
		return
	}
	x, e := h.sharing.AcceptInvite(r.Context(), in.Token, in.Username, in.DisplayName, in.Password, address, RequestID(r.Context()), in.Device)
	if e != nil {
		h.sharingError(w, r, e)
		return
	}
	_ = h.auth.Audit(r.Context(), "INVITATION_ACCEPTED", &x.User.ID, "invitation", "", RequestID(r.Context()), map[string]any{"role": x.User.Role})
	h.sendTokens(w, r, x, 201)
}
func (h *Handler) invitations(w http.ResponseWriter, r *http.Request, _ auth.Principal) {
	x, e := h.sharing.Invitations(r.Context())
	if e != nil {
		h.sharingError(w, r, e)
		return
	}
	writeJSON(w, 200, map[string]any{"invitations": x})
}
func (h *Handler) createInvitation(w http.ResponseWriter, r *http.Request, p auth.Principal) {
	var in struct {
		Identifier     string
		Role           auth.Role
		Libraries      []sharing.Grant
		ExpiresInHours int
	}
	if !decode(w, r, &in) {
		return
	}
	x, token, e := h.sharing.CreateInvite(r.Context(), p, in.Identifier, in.Role, in.Libraries, time.Duration(in.ExpiresInHours)*time.Hour)
	if e != nil {
		h.sharingError(w, r, e)
		return
	}
	_ = h.auth.Audit(r.Context(), "INVITATION_CREATED", &p.UserID, "invitation", x.ID, RequestID(r.Context()), map[string]any{"role": x.Role, "expiresAt": x.ExpiresAt})
	writeJSON(w, 201, map[string]any{"invitation": x, "invitePath": "/invite/" + token, "shownOnce": true})
}
func (h *Handler) revokeInvitation(w http.ResponseWriter, r *http.Request, p auth.Principal) {
	if e := h.sharing.RevokeInvite(r.Context(), r.PathValue("id")); e != nil {
		h.sharingError(w, r, e)
		return
	}
	_ = h.auth.Audit(r.Context(), "INVITATION_REVOKED", &p.UserID, "invitation", r.PathValue("id"), RequestID(r.Context()), map[string]any{})
	w.WriteHeader(204)
}
func (h *Handler) userGrants(w http.ResponseWriter, r *http.Request, _ auth.Principal) {
	x, e := h.sharing.Grants(r.Context(), r.PathValue("id"))
	if e != nil {
		h.sharingError(w, r, e)
		return
	}
	writeJSON(w, 200, map[string]any{"grants": x})
}
func (h *Handler) setUserGrants(w http.ResponseWriter, r *http.Request, p auth.Principal) {
	var in struct{ Grants []sharing.Grant }
	if !decode(w, r, &in) {
		return
	}
	if e := h.sharing.SetGrants(r.Context(), p.UserID, r.PathValue("id"), in.Grants); e != nil {
		h.sharingError(w, r, e)
		return
	}
	_ = h.auth.Audit(r.Context(), "LIBRARY_ACCESS_UPDATED", &p.UserID, "user", r.PathValue("id"), RequestID(r.Context()), map[string]any{"grantCount": len(in.Grants)})
	w.WriteHeader(204)
}
func (h *Handler) createPairing(w http.ResponseWriter, r *http.Request) {
	address, _, _ := h.connection(r)
	if !authLimiter.allow("pair-create:" + address) {
		h.sharingError(w, r, sharing.ErrLimited)
		return
	}
	var d auth.DeviceInput
	if !decode(w, r, &d) {
		return
	}
	x, e := h.sharing.CreatePairing(r.Context(), d)
	if e != nil {
		h.sharingError(w, r, e)
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	writeJSON(w, 201, x)
}
func (h *Handler) pairingStatus(w http.ResponseWriter, r *http.Request) {
	address, _, _ := h.connection(r)
	if !authLimiter.allow("pair-status:" + address) {
		h.sharingError(w, r, sharing.ErrLimited)
		return
	}
	x, e := h.sharing.PairingStatus(r.Context(), r.PathValue("id"))
	if e != nil {
		h.sharingError(w, r, e)
		return
	}
	writeJSON(w, 200, map[string]any{"id": x.ID, "status": x.Status, "expiresAt": x.ExpiresAt, "pollAfterSeconds": 3})
}
func (h *Handler) lookupPairing(w http.ResponseWriter, r *http.Request, _ auth.Principal) {
	var in struct{ Code string }
	if !decode(w, r, &in) {
		return
	}
	address, _, _ := h.connection(r)
	x, e := h.sharing.PairingByCode(r.Context(), strings.ToUpper(in.Code), address)
	if e != nil {
		h.sharingError(w, r, e)
		return
	}
	writeJSON(w, 200, x)
}
func (h *Handler) approvePairing(w http.ResponseWriter, r *http.Request, p auth.Principal) {
	if e := h.sharing.DecidePairing(r.Context(), r.PathValue("id"), p.UserID, true); e != nil {
		h.sharingError(w, r, e)
		return
	}
	_ = h.auth.Audit(r.Context(), "DEVICE_PAIRING_APPROVED", &p.UserID, "pairing", r.PathValue("id"), RequestID(r.Context()), map[string]any{})
	w.WriteHeader(204)
}
func (h *Handler) denyPairing(w http.ResponseWriter, r *http.Request, p auth.Principal) {
	if e := h.sharing.DecidePairing(r.Context(), r.PathValue("id"), p.UserID, false); e != nil {
		h.sharingError(w, r, e)
		return
	}
	_ = h.auth.Audit(r.Context(), "DEVICE_PAIRING_DENIED", &p.UserID, "pairing", r.PathValue("id"), RequestID(r.Context()), map[string]any{})
	w.WriteHeader(204)
}
func (h *Handler) exchangePairing(w http.ResponseWriter, r *http.Request) {
	address, _, _ := h.connection(r)
	if !authLimiter.allow("pair-exchange:" + address) {
		h.sharingError(w, r, sharing.ErrLimited)
		return
	}
	var in struct{ Challenge string }
	if !decode(w, r, &in) {
		return
	}
	x, e := h.sharing.ExchangePairing(r.Context(), r.PathValue("id"), in.Challenge, address, RequestID(r.Context()))
	if e != nil {
		h.sharingError(w, r, e)
		return
	}
	r.Header.Set("X-VyNode-Client", "native")
	h.sendTokens(w, r, x, 200)
}
func (h *Handler) remoteAccess(w http.ResponseWriter, r *http.Request, _ auth.Principal) {
	x, e := h.sharing.Remote(r.Context())
	if e != nil {
		h.sharingError(w, r, e)
		return
	}
	writeJSON(w, 200, x)
}
func (h *Handler) saveRemoteAccess(w http.ResponseWriter, r *http.Request, p auth.Principal) {
	var x sharing.RemoteSettings
	if !decode(w, r, &x) {
		return
	}
	if e := h.sharing.SaveRemote(r.Context(), x); e != nil {
		h.sharingError(w, r, e)
		return
	}
	_ = h.auth.Audit(r.Context(), "REMOTE_ACCESS_CONFIGURATION_CHANGED", &p.UserID, "server", "remote-access", RequestID(r.Context()), map[string]any{"discoveryEnabled": x.DiscoveryEnabled, "portMappingEnabled": x.PortMappingEnabled, "reverseProxyEnabled": x.ReverseProxyEnabled})
	h.remoteAccess(w, r, p)
}
