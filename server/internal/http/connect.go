package httpserver

import (
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"github.com/vynode/media/server/internal/auth"
	connectservice "github.com/vynode/media/server/internal/connect"
)

func (h *Handler) connectRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/v1/connect/settings", h.require(auth.CapUsersManage, h.connectSettings))
	mux.HandleFunc("PUT /api/v1/connect/settings", h.require(auth.CapUsersManage, h.saveConnectSettings))
	mux.HandleFunc("POST /api/v1/connect/server/claim", h.require(auth.CapUsersManage, h.beginServerClaim))
	mux.HandleFunc("DELETE /api/v1/connect/server", h.require(auth.CapUsersManage, h.unlinkServer))
	mux.HandleFunc("PUT /api/v1/connect/server/endpoint", h.require(auth.CapUsersManage, h.publishConnectEndpoint))
	mux.HandleFunc("POST /api/v1/connect/link/request", h.protected(h.connectLinkRequest))
	mux.HandleFunc("POST /api/v1/connect/link/complete", h.protected(h.connectLinkComplete))
	mux.HandleFunc("DELETE /api/v1/connect/link", h.protected(h.connectUnlink))
	mux.HandleFunc("POST /api/v1/connect/exchange", h.connectExchange)
	mux.HandleFunc("POST /api/v1/connect/invitations", h.require(auth.CapUsersManage, h.createGlobalInvitation))
	mux.HandleFunc("POST /api/v1/connect/invitations/redeem", h.redeemGlobalInvitation)
	mux.HandleFunc("DELETE /api/v1/connect/invitations/{id}", h.require(auth.CapUsersManage, h.revokeGlobalInvitation))
}
func (h *Handler) createGlobalInvitation(w http.ResponseWriter, r *http.Request, p auth.Principal) {
	var in struct {
		IntendedUsername string                           `json:"intendedUsername"`
		DisplayName      string                           `json:"displayName"`
		Role             auth.Role                        `json:"role"`
		Grants           []connectservice.InvitationGrant `json:"grants"`
		ExpiresInSeconds int64                            `json:"expiresInSeconds"`
	}
	if !decode(w, r, &in) {
		return
	}
	value, err := h.connect.CreateGlobalInvitation(r.Context(), p, in.IntendedUsername, in.DisplayName, in.Role, in.Grants, time.Duration(in.ExpiresInSeconds)*time.Second)
	if err != nil {
		writeError(w, r, 409, "CONNECT_INVITATION_FAILED", "The global invitation could not be created.")
		return
	}
	writeJSON(w, 201, value)
}
func (h *Handler) redeemGlobalInvitation(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Redemption string `json:"redemption"`
	}
	if !decode(w, r, &in) {
		return
	}
	if err := h.connect.RedeemGlobalInvitation(r.Context(), in.Redemption, RequestID(r.Context())); err != nil {
		writeError(w, r, 409, "CONNECT_INVITATION_REDEMPTION_INVALID", "The invitation redemption is invalid, expired, revoked, or already used.")
		return
	}
	writeJSON(w, 200, map[string]bool{"provisioned": true})
}
func (h *Handler) revokeGlobalInvitation(w http.ResponseWriter, r *http.Request, p auth.Principal) {
	if err := h.connect.RevokeGlobalInvitation(r.Context(), p, r.PathValue("id")); err != nil {
		writeError(w, r, 404, "CONNECT_INVITATION_NOT_FOUND", "The invitation was not found or is no longer pending.")
		return
	}
	w.WriteHeader(204)
}
func (h *Handler) publishConnectEndpoint(w http.ResponseWriter, r *http.Request, _ auth.Principal) {
	var in struct {
		URL  string `json:"url"`
		Kind string `json:"kind"`
	}
	if !decode(w, r, &in) {
		return
	}
	if err := h.connect.PublishEndpoint(r.Context(), in.URL, in.Kind); err != nil {
		writeError(w, r, 502, "CONNECT_ENDPOINT_FAILED", "The signed endpoint could not be published.")
		return
	}
	w.WriteHeader(204)
}
func (h *Handler) unlinkServer(w http.ResponseWriter, r *http.Request, _ auth.Principal) {
	if err := h.connect.UnlinkServer(r.Context()); err != nil {
		writeError(w, r, 500, "CONNECT_UNLINK_FAILED", "The local Connect relationship could not be removed.")
		return
	}
	w.WriteHeader(204)
}
func (h *Handler) beginServerClaim(w http.ResponseWriter, r *http.Request, _ auth.Principal) {
	v, err := h.connect.RegisterAndBeginClaim(r.Context(), h.info.ServerName, h.info.Version)
	if err != nil {
		writeError(w, r, 502, "CONNECT_REGISTRATION_FAILED", "The server could not register with Connect.")
		return
	}
	writeJSON(w, 200, map[string]string{"challenge": v})
}
func (h *Handler) connectSettings(w http.ResponseWriter, r *http.Request, _ auth.Principal) {
	v, err := h.connect.Settings(r.Context())
	if err != nil {
		writeError(w, r, 500, "INTERNAL_ERROR", "Unable to read Connect settings.")
		return
	}
	writeJSON(w, 200, v)
}
func (h *Handler) saveConnectSettings(w http.ResponseWriter, r *http.Request, _ auth.Principal) {
	var in struct {
		Enabled     bool            `json:"enabled"`
		ConnectURL  string          `json:"connectUrl"`
		Issuer      string          `json:"issuer"`
		SigningKeys json.RawMessage `json:"signingKeys"`
	}
	if !decode(w, r, &in) {
		return
	}
	if err := h.connect.Configure(r.Context(), in.Enabled, in.ConnectURL, in.Issuer, in.SigningKeys); err != nil {
		writeError(w, r, 400, "VALIDATION_FAILED", "Connect settings are invalid.")
		return
	}
	w.WriteHeader(204)
}
func (h *Handler) connectLinkRequest(w http.ResponseWriter, r *http.Request, p auth.Principal) {
	v, err := h.connect.CreateLinkRequest(r.Context(), p)
	if err != nil {
		writeError(w, r, 409, "CONNECT_LINK_UNAVAILABLE", "Connect linking is unavailable.")
		return
	}
	writeJSON(w, 201, v)
}
func (h *Handler) connectLinkComplete(w http.ResponseWriter, r *http.Request, p auth.Principal) {
	var in struct {
		State string `json:"state"`
		Grant string `json:"grant"`
	}
	if !decode(w, r, &in) {
		return
	}
	if err := h.connect.CompleteLink(r.Context(), p, in.State, in.Grant); err != nil {
		writeError(w, r, 409, "CONNECT_LINK_INVALID", "The link authorization is invalid, expired, already used, or belongs to another session.")
		return
	}
	writeJSON(w, 200, map[string]bool{"linked": true})
}
func (h *Handler) connectUnlink(w http.ResponseWriter, r *http.Request, p auth.Principal) {
	if err := h.connect.Unlink(r.Context(), p); err != nil {
		writeError(w, r, 404, "CONNECT_LINK_NOT_FOUND", "No active global link exists.")
		return
	}
	w.WriteHeader(204)
}
func (h *Handler) connectExchange(w http.ResponseWriter, r *http.Request) {
	address, _, _ := h.connection(r)
	var in struct {
		Assertion string           `json:"assertion"`
		Device    auth.DeviceInput `json:"device"`
	}
	if !decode(w, r, &in) {
		return
	}
	tokens, err := h.connect.Exchange(r.Context(), in.Assertion, in.Device, address, RequestID(r.Context()))
	if err != nil {
		if errors.Is(err, connectservice.ErrUnlinked) {
			writeError(w, r, 403, "CONNECT_ACCOUNT_UNLINKED", "This global account is not linked to an active local user.")
			return
		}
		writeError(w, r, 401, "CONNECT_ASSERTION_INVALID", "The Connect assertion is invalid or has already been used.")
		return
	}
	h.sendTokens(w, r, tokens, 200)
}
