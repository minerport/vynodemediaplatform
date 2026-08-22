package httpserver

import (
	"encoding/json"
	"errors"
	"github.com/vynode/media/server/internal/auth"
	"net"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"
)

type limiter struct {
	mu       sync.Mutex
	attempts map[string][]time.Time
}

var authLimiter = &limiter{attempts: map[string][]time.Time{}}

func (l *limiter) allow(key string) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	cut := time.Now().Add(-time.Minute)
	recent := l.attempts[key][:0]
	for _, t := range l.attempts[key] {
		if t.After(cut) {
			recent = append(recent, t)
		}
	}
	if len(recent) >= 10 {
		l.attempts[key] = recent
		return false
	}
	l.attempts[key] = append(recent, time.Now())
	return true
}
func (h *Handler) authRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/v1/setup/status", h.setupStatus)
	mux.HandleFunc("POST /api/v1/setup/owner", h.setupOwner)
	mux.HandleFunc("POST /api/v1/auth/login", h.login)
	mux.HandleFunc("POST /api/v1/auth/refresh", h.refresh)
	mux.HandleFunc("POST /api/v1/auth/logout", h.protected(h.logout))
	mux.HandleFunc("POST /api/v1/auth/logout-others", h.protected(h.logoutOthers))
	mux.HandleFunc("GET /api/v1/account/me", h.protected(h.me))
	mux.HandleFunc("GET /api/v1/account/sessions", h.protected(h.sessions))
	mux.HandleFunc("DELETE /api/v1/account/sessions/{sessionId}", h.protected(h.revokeSession))
	mux.HandleFunc("POST /api/v1/account/password", h.protected(h.changePassword))
	mux.HandleFunc("GET /api/v1/admin/users", h.require(auth.CapUsersManage, h.listUsers))
	mux.HandleFunc("POST /api/v1/admin/users", h.require(auth.CapUsersManage, h.createUser))
	mux.HandleFunc("GET /api/v1/admin/users/{id}", h.require(auth.CapUsersManage, h.getUser))
	mux.HandleFunc("PATCH /api/v1/admin/users/{id}", h.require(auth.CapUsersManage, h.setUser))
	mux.HandleFunc("GET /api/v1/admin/audit", h.require(auth.CapAuditView, h.auditPage))
}
func decode(w http.ResponseWriter, r *http.Request, v any) bool {
	r.Body = http.MaxBytesReader(w, r.Body, 16<<10)
	d := json.NewDecoder(r.Body)
	d.DisallowUnknownFields()
	if d.Decode(v) != nil {
		writeError(w, r, 400, "VALIDATION_FAILED", "The request is invalid.")
		return false
	}
	return true
}
func remoteIP(r *http.Request) string {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err == nil {
		return host
	}
	return r.RemoteAddr
}
func (h *Handler) connection(r *http.Request) (string, bool, bool) {
	if h.sharing == nil {
		return remoteIP(r), r.TLS != nil, false
	}
	c := h.sharing.ResolveConnection(r.Context(), r.RemoteAddr, r.Header.Get("X-Forwarded-For"), r.Header.Get("X-Forwarded-Proto"), r.TLS != nil)
	return c.Address, c.Secure, c.Local
}
func (h *Handler) setupStatus(w http.ResponseWriter, r *http.Request) {
	required, err := h.auth.SetupRequired(r.Context())
	if err != nil {
		writeError(w, r, 500, "INTERNAL_ERROR", "An unexpected error occurred.")
		return
	}
	writeJSON(w, 200, map[string]bool{"setupRequired": required})
}
func (h *Handler) setupOwner(w http.ResponseWriter, r *http.Request) {
	address, _, _ := h.connection(r)
	if !authLimiter.allow("setup:" + address) {
		writeError(w, r, 429, "RATE_LIMITED", "Too many attempts. Try again shortly.")
		return
	}
	var in struct {
		Username, DisplayName, Password, ServerName string
		Device                                      auth.DeviceInput
	}
	if !decode(w, r, &in) {
		return
	}
	tokens, err := h.auth.Bootstrap(r.Context(), in.Username, in.DisplayName, in.Password, in.ServerName, RequestID(r.Context()), address, in.Device)
	if err != nil {
		h.authError(w, r, err)
		return
	}
	h.sendTokens(w, r, tokens, http.StatusCreated)
}
func (h *Handler) login(w http.ResponseWriter, r *http.Request) {
	address, _, _ := h.connection(r)
	if !authLimiter.allow("login:" + address) {
		writeError(w, r, 429, "RATE_LIMITED", "Too many attempts. Try again shortly.")
		return
	}
	var in struct {
		Username, Password string
		Device             auth.DeviceInput
	}
	if !decode(w, r, &in) {
		return
	}
	tokens, err := h.auth.Login(r.Context(), in.Username, in.Password, RequestID(r.Context()), address, in.Device)
	if err != nil {
		h.authError(w, r, err)
		return
	}
	h.sendTokens(w, r, tokens, 200)
}
func (h *Handler) refresh(w http.ResponseWriter, r *http.Request) {
	address, _, _ := h.connection(r)
	if !authLimiter.allow("refresh:" + address) {
		writeError(w, r, 429, "RATE_LIMITED", "Too many attempts. Try again shortly.")
		return
	}
	if !h.validOrigin(r) {
		writeError(w, r, 403, "FORBIDDEN", "Request origin is not allowed.")
		return
	}
	var in struct {
		RefreshToken string `json:"refreshToken"`
	}
	_ = json.NewDecoder(http.MaxBytesReader(w, r.Body, 4<<10)).Decode(&in)
	if in.RefreshToken == "" {
		if c, err := r.Cookie("vynode_refresh"); err == nil {
			in.RefreshToken = c.Value
		}
	}
	tokens, err := h.auth.Refresh(r.Context(), in.RefreshToken, RequestID(r.Context()))
	if err != nil {
		clearRefresh(w)
		h.authError(w, r, err)
		return
	}
	h.sendTokens(w, r, tokens, 200)
}
func (h *Handler) sendTokens(w http.ResponseWriter, r *http.Request, t auth.Tokens, status int) {
	if r.Header.Get("X-VyNode-Client") == "native" {
		writeJSON(w, status, t)
		return
	}
	_, secure, _ := h.connection(r)
	http.SetCookie(w, &http.Cookie{Name: "vynode_refresh", Value: t.RefreshToken, Path: "/api/v1/auth", HttpOnly: true, Secure: secure, SameSite: http.SameSiteStrictMode, MaxAge: int(h.auth.RefreshTTL.Seconds())})
	t.RefreshToken = ""
	writeJSON(w, status, t)
}
func clearRefresh(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{Name: "vynode_refresh", Value: "", Path: "/api/v1/auth", HttpOnly: true, SameSite: http.SameSiteStrictMode, MaxAge: -1})
}
func (h *Handler) validOrigin(r *http.Request) bool {
	origin := r.Header.Get("Origin")
	if origin == "" {
		return true
	}
	if h.allowedOrigin != "" {
		return origin == h.allowedOrigin
	}
	return origin == "http://"+r.Host || origin == "https://"+r.Host
}
func (h *Handler) originGuard(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		origin := r.Header.Get("Origin")
		if origin != "" && h.validOrigin(r) {
			w.Header().Set("Access-Control-Allow-Origin", origin)
			w.Header().Set("Access-Control-Allow-Credentials", "true")
			w.Header().Add("Vary", "Origin")
		}
		if r.Method == http.MethodOptions {
			if origin == "" || !h.validOrigin(r) {
				writeError(w, r, http.StatusForbidden, "FORBIDDEN", "Request origin is not allowed.")
				return
			}
			w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PATCH, DELETE, OPTIONS")
			w.Header().Set("Access-Control-Allow-Headers", "Authorization, Content-Type, X-Request-ID")
			w.WriteHeader(http.StatusNoContent)
			return
		}
		if r.Method != http.MethodGet && r.Method != http.MethodHead && origin != "" && !h.validOrigin(r) {
			writeError(w, r, http.StatusForbidden, "FORBIDDEN", "Request origin is not allowed.")
			return
		}
		next.ServeHTTP(w, r)
	})
}
func bearer(r *http.Request) string {
	v := r.Header.Get("Authorization")
	p := strings.SplitN(v, " ", 2)
	if len(p) == 2 && p[0] == "Bearer" {
		return p[1]
	}
	return ""
}
func (h *Handler) protected(next func(http.ResponseWriter, *http.Request, auth.Principal)) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		p, err := h.auth.Authenticate(bearer(r))
		if err != nil {
			writeError(w, r, 401, "AUTHENTICATION_REQUIRED", "Authentication is required.")
			return
		}
		next(w, r, p)
	}
}
func (h *Handler) require(cap auth.Capability, next func(http.ResponseWriter, *http.Request, auth.Principal)) http.HandlerFunc {
	return h.protected(func(w http.ResponseWriter, r *http.Request, p auth.Principal) {
		if !auth.Allowed(p.Role, cap) {
			writeError(w, r, 403, "FORBIDDEN", "You are not authorized for this operation.")
			return
		}
		next(w, r, p)
	})
}
func (h *Handler) me(w http.ResponseWriter, r *http.Request, p auth.Principal) {
	u, err := h.auth.Me(r.Context(), p)
	if err != nil {
		h.authError(w, r, err)
		return
	}
	writeJSON(w, 200, u)
}
func (h *Handler) sessions(w http.ResponseWriter, r *http.Request, p auth.Principal) {
	v, err := h.auth.Sessions(r.Context(), p)
	if err != nil {
		h.authError(w, r, err)
		return
	}
	writeJSON(w, 200, map[string]any{"sessions": v})
}
func (h *Handler) revokeSession(w http.ResponseWriter, r *http.Request, p auth.Principal) {
	if err := h.auth.Revoke(r.Context(), p, r.PathValue("sessionId"), RequestID(r.Context()), "SESSION_REVOKED"); err != nil {
		h.authError(w, r, err)
		return
	}
	w.WriteHeader(204)
}
func (h *Handler) logout(w http.ResponseWriter, r *http.Request, p auth.Principal) {
	if !h.validOrigin(r) {
		writeError(w, r, 403, "FORBIDDEN", "Request origin is not allowed.")
		return
	}
	if err := h.auth.Revoke(r.Context(), p, p.SessionID, RequestID(r.Context()), "USER_LOGGED_OUT"); err != nil {
		h.authError(w, r, err)
		return
	}
	clearRefresh(w)
	w.WriteHeader(204)
}
func (h *Handler) logoutOthers(w http.ResponseWriter, r *http.Request, p auth.Principal) {
	if err := h.auth.RevokeOthers(r.Context(), p, RequestID(r.Context())); err != nil {
		h.authError(w, r, err)
		return
	}
	w.WriteHeader(204)
}
func (h *Handler) changePassword(w http.ResponseWriter, r *http.Request, p auth.Principal) {
	var in struct {
		CurrentPassword string `json:"currentPassword"`
		NewPassword     string `json:"newPassword"`
	}
	if !decode(w, r, &in) {
		return
	}
	if err := h.auth.ChangePassword(r.Context(), p, in.CurrentPassword, in.NewPassword, RequestID(r.Context())); err != nil {
		h.authError(w, r, err)
		return
	}
	w.WriteHeader(204)
}
func (h *Handler) listUsers(w http.ResponseWriter, r *http.Request, p auth.Principal) {
	v, err := h.auth.ListUsers(r.Context())
	if err != nil {
		h.authError(w, r, err)
		return
	}
	writeJSON(w, 200, map[string]any{"users": v})
}
func (h *Handler) getUser(w http.ResponseWriter, r *http.Request, p auth.Principal) {
	u, err := h.auth.GetUser(r.Context(), r.PathValue("id"))
	if err != nil {
		h.authError(w, r, err)
		return
	}
	writeJSON(w, 200, u)
}
func (h *Handler) createUser(w http.ResponseWriter, r *http.Request, p auth.Principal) {
	var in struct {
		Username, DisplayName, Password string
		Role                            auth.Role
	}
	if !decode(w, r, &in) {
		return
	}
	u, err := h.auth.CreateUser(r.Context(), p, in.Username, in.DisplayName, in.Password, in.Role, RequestID(r.Context()))
	if err != nil {
		h.authError(w, r, err)
		return
	}
	writeJSON(w, 201, u)
}
func (h *Handler) setUser(w http.ResponseWriter, r *http.Request, p auth.Principal) {
	var in struct {
		Enabled *bool `json:"enabled"`
	}
	if !decode(w, r, &in) || in.Enabled == nil {
		return
	}
	if err := h.auth.SetEnabled(r.Context(), p, r.PathValue("id"), *in.Enabled, RequestID(r.Context())); err != nil {
		h.authError(w, r, err)
		return
	}
	w.WriteHeader(204)
}
func (h *Handler) auditPage(w http.ResponseWriter, r *http.Request, p auth.Principal) {
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	offset, _ := strconv.Atoi(r.URL.Query().Get("offset"))
	v, err := h.auth.AuditPage(r.Context(), limit, offset)
	if err != nil {
		h.authError(w, r, err)
		return
	}
	writeJSON(w, 200, map[string]any{"events": v, "limit": limit, "offset": offset})
}
func (h *Handler) authError(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, auth.ErrValidation):
		writeError(w, r, 400, "VALIDATION_FAILED", "The request is invalid.")
	case errors.Is(err, auth.ErrSetupComplete):
		writeError(w, r, 409, "SETUP_ALREADY_COMPLETE", "Initial setup is already complete.")
	case errors.Is(err, auth.ErrInvalidCredentials):
		writeError(w, r, 401, "INVALID_CREDENTIALS", "The username or password is invalid.")
	case errors.Is(err, auth.ErrUnauthorized):
		writeError(w, r, 401, "AUTHENTICATION_REQUIRED", "Authentication is required.")
	case errors.Is(err, auth.ErrForbidden):
		writeError(w, r, 403, "FORBIDDEN", "You are not authorized for this operation.")
	case errors.Is(err, auth.ErrSessionRevoked):
		writeError(w, r, 401, "SESSION_REVOKED", "The session is no longer valid.")
	case errors.Is(err, auth.ErrUsernameExists):
		writeError(w, r, 409, "USERNAME_ALREADY_EXISTS", "That username is unavailable.")
	default:
		writeError(w, r, 500, "INTERNAL_ERROR", "An unexpected error occurred.")
	}
}
