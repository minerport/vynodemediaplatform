package httpapi

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/vynode/media/connect/internal/account"
	"github.com/vynode/media/connect/internal/registry"
	"github.com/vynode/media/connect/internal/store"
)

type Handler struct {
	accounts               *account.Service
	registry               *registry.Service
	store                  *store.Store
	mu                     sync.Mutex
	attempts               map[string][]time.Time
	allowedOrigin          string
	allowInsecureEndpoints bool
}

func New(a *account.Service, r *registry.Service, s *store.Store, allowedOrigin string, allowInsecureEndpoints bool) http.Handler {
	h := &Handler{accounts: a, registry: r, store: s, attempts: map[string][]time.Time{}, allowedOrigin: strings.TrimSpace(allowedOrigin), allowInsecureEndpoints: allowInsecureEndpoints}
	m := http.NewServeMux()
	m.HandleFunc("/health/live", func(w http.ResponseWriter, _ *http.Request) { write(w, 200, map[string]string{"status": "ok"}) })
	m.HandleFunc("/health/ready", h.ready)
	m.HandleFunc("/.well-known/vynode-connect-keys", h.keys)
	m.HandleFunc("/api/v1/", h.api)
	m.Handle("/", webAssets())
	return security(h.cors(m))
}
func (h *Handler) cors(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		origin := r.Header.Get("Origin")
		scheme := "http"
		if r.TLS != nil {
			scheme = "https"
		}
		allowed := origin == "" || origin == scheme+"://"+r.Host || (h.allowedOrigin != "" && origin == h.allowedOrigin)
		if origin != "" && allowed {
			w.Header().Set("Access-Control-Allow-Origin", origin)
			w.Header().Set("Access-Control-Allow-Credentials", "true")
			w.Header().Add("Vary", "Origin")
		}
		if r.Method == "OPTIONS" {
			if origin == "" || !allowed {
				problem(w, 403, "forbidden")
				return
			}
			w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
			w.Header().Set("Access-Control-Allow-Headers", "Authorization, Content-Type, X-Request-ID, X-VyNode-Client")
			w.WriteHeader(204)
			return
		}
		if !allowed {
			problem(w, 403, "forbidden")
			return
		}
		next.ServeHTTP(w, r)
	})
}
func security(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("Referrer-Policy", "no-referrer")
		w.Header().Set("Content-Security-Policy", "default-src 'self'; frame-ancestors 'none'; object-src 'none'; base-uri 'none'; form-action 'self'")
		next.ServeHTTP(w, r)
	})
}
func (h *Handler) ready(w http.ResponseWriter, r *http.Request) {
	if h.store.Ready(r.Context()) != nil {
		problem(w, 503, "not_ready")
		return
	}
	write(w, 200, map[string]string{"status": "ready"})
}
func (h *Handler) keys(w http.ResponseWriter, r *http.Request) {
	v, err := h.registry.PublicKeys(r.Context())
	if err != nil {
		problem(w, 500, "internal_error")
		return
	}
	write(w, 200, v)
}
func (h *Handler) principal(r *http.Request) (account.Principal, error) {
	v := strings.TrimSpace(strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer "))
	return h.accounts.Authenticate(v)
}
func (h *Handler) limited(key string) bool {
	now := time.Now()
	h.mu.Lock()
	defer h.mu.Unlock()
	v := h.attempts[key][:0]
	for _, t := range h.attempts[key] {
		if now.Sub(t) < time.Minute {
			v = append(v, t)
		}
	}
	if len(v) >= 10 {
		h.attempts[key] = v
		return true
	}
	h.attempts[key] = append(v, now)
	return false
}
func (h *Handler) api(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.Path, "/api/v1")
	requestID := r.Header.Get("X-Request-ID")
	var body map[string]any
	if r.Body != nil {
		_ = json.NewDecoder(http.MaxBytesReader(w, r.Body, 64<<10)).Decode(&body)
	}
	str := func(k string) string { v, _ := body[k].(string); return v }
	device := func() account.DeviceInput {
		return account.DeviceInput{Name: str("deviceName"), Platform: str("platform"), ClientName: str("clientName"), ClientVersion: str("clientVersion"), PlatformVersion: str("platformVersion")}
	}
	if path == "/account/register" && r.Method == "POST" {
		if h.limited("register:" + r.RemoteAddr) {
			problem(w, 429, "rate_limited")
			return
		}
		v, e := h.accounts.Register(r.Context(), str("username"), str("displayName"), str("password"), device(), requestID)
		h.respondTokens(w, r, v, e)
		return
	}
	if path == "/account/login" && r.Method == "POST" {
		if h.limited("login:" + r.RemoteAddr) {
			problem(w, 429, "rate_limited")
			return
		}
		v, e := h.accounts.Login(r.Context(), str("username"), str("password"), device(), requestID)
		h.respondTokens(w, r, v, e)
		return
	}
	if path == "/account/refresh" && r.Method == "POST" {
		if h.limited("refresh:" + r.RemoteAddr) {
			problem(w, 429, "rate_limited")
			return
		}
		token := str("refreshToken")
		if token == "" {
			if cookie, err := r.Cookie("vynode_connect_refresh"); err == nil {
				token = cookie.Value
			}
		}
		v, e := h.accounts.Refresh(r.Context(), token, requestID)
		h.respondTokens(w, r, v, e)
		return
	}
	if path == "/device-codes" && r.Method == "POST" {
		if h.limited("device-code:" + r.RemoteAddr) {
			problem(w, 429, "rate_limited")
			return
		}
		v, e := h.registry.CreateDeviceCode(r.Context(), device())
		respond(w, v, e)
		return
	}
	if path == "/device-codes/exchange" && r.Method == "POST" {
		if h.limited("device-exchange:" + r.RemoteAddr) {
			problem(w, 429, "rate_limited")
			return
		}
		aid, d, e := h.registry.ExchangeDeviceCode(r.Context(), str("deviceCode"))
		if e != nil {
			respond(w, nil, e)
			return
		}
		v, e := h.accounts.IssueForAccount(r.Context(), aid, d, requestID)
		h.respondTokens(w, r, v, e)
		return
	}
	if path == "/servers/register" && r.Method == "POST" {
		v, x := h.registry.RegisterServer(r.Context(), str("serverId"), str("name"), str("publicKey"), str("version"), str("signature"))
		respond(w, v, x)
		return
	}
	if path == "/servers/claim" && r.Method == "POST" {
		if h.limited("server-claim:" + r.RemoteAddr) {
			problem(w, 429, "rate_limited")
			return
		}
		v, x := h.registry.CreateClaim(r.Context(), str("serverId"), str("signature"))
		respond(w, map[string]string{"challenge": v}, x)
		return
	}
	if path == "/servers/endpoint" && r.Method == "PUT" {
		x := h.registry.UpdateEndpoint(r.Context(), str("serverId"), str("url"), str("kind"), str("signature"), h.allowInsecureEndpoints)
		respond(w, map[string]bool{"ok": x == nil}, x)
		return
	}
	if path == "/servers/heartbeat" && r.Method == "POST" {
		x := h.registry.Heartbeat(r.Context(), str("serverId"), str("version"), str("signature"))
		respond(w, map[string]bool{"ok": x == nil}, x)
		return
	}
	if path == "/servers/device-revocations" && r.Method == "POST" {
		v, x := h.registry.DeviceRevocations(r.Context(), str("serverId"), str("signature"))
		respond(w, v, x)
		return
	}
	if path == "/servers/invitations/register" && r.Method == "POST" {
		if h.limited("invitation-register:" + r.RemoteAddr) {
			problem(w, 429, "rate_limited")
			return
		}
		v, x := h.registry.RegisterInvitation(r.Context(), str("serverId"), str("invitationId"), str("intendedUsername"), str("intentDigest"), str("expiresAt"), str("signature"))
		respond(w, v, x)
		return
	}
	if path == "/servers/invitations/revoke" && r.Method == "POST" {
		x := h.registry.RevokeRegisteredInvitation(r.Context(), str("serverId"), str("invitationId"), str("signature"))
		respond(w, map[string]bool{"ok": x == nil}, x)
		return
	}
	p, e := h.principal(r)
	if e != nil {
		problem(w, 401, "authentication_required")
		return
	}
	switch {
	case path == "/account/logout" && r.Method == "POST":
		e = h.accounts.Logout(r.Context(), p)
		http.SetCookie(w, &http.Cookie{Name: "vynode_connect_refresh", Value: "", Path: "/api/v1/account", HttpOnly: true, SameSite: http.SameSiteStrictMode, MaxAge: -1})
		respond(w, map[string]bool{"ok": e == nil}, e)
	case path == "/account/me" && r.Method == "GET":
		v, x := h.accounts.Me(r.Context(), p)
		respond(w, v, x)
	case path == "/devices" && r.Method == "GET":
		v, x := h.accounts.Devices(r.Context(), p)
		respond(w, v, x)
	case strings.HasPrefix(path, "/devices/") && r.Method == "DELETE":
		e = h.accounts.RevokeDevice(r.Context(), p, strings.TrimPrefix(path, "/devices/"), requestID)
		respond(w, map[string]bool{"ok": e == nil}, e)
	case path == "/servers" && r.Method == "GET":
		v, x := h.registry.Servers(r.Context(), p)
		respond(w, v, x)
	case path == "/servers/claim/complete" && r.Method == "POST":
		v, x := h.registry.CompleteClaim(r.Context(), p, str("challenge"))
		respond(w, v, x)
	case strings.HasSuffix(path, "/assertion") && r.Method == "POST":
		id := strings.TrimSuffix(strings.TrimPrefix(path, "/servers/"), "/assertion")
		v, x := h.registry.Assertion(r.Context(), p, id)
		respond(w, map[string]string{"assertion": v}, x)
	case strings.HasSuffix(path, "/link-grant") && r.Method == "POST":
		id := strings.TrimSuffix(strings.TrimPrefix(path, "/servers/"), "/link-grant")
		v, x := h.registry.LinkGrant(r.Context(), p, id, str("state"))
		respond(w, map[string]string{"grant": v}, x)
	case strings.HasSuffix(path, "/unlink") && r.Method == "POST":
		id := strings.TrimSuffix(strings.TrimPrefix(path, "/servers/"), "/unlink")
		x := h.registry.Unlink(r.Context(), p, id)
		respond(w, map[string]bool{"ok": x == nil}, x)
	case path == "/invitations" && r.Method == "POST":
		v, x := h.registry.CreateInvitation(r.Context(), p, str("serverId"), str("intendedUsername"), str("localPrincipalHint"), 7*24*time.Hour)
		respond(w, v, x)
	case path == "/invitations/accept" && r.Method == "POST":
		v, x := h.registry.AcceptInvitationRedemption(r.Context(), p, str("token"))
		respond(w, v, x)
	case path == "/device-codes/approve" && r.Method == "POST":
		if h.limited("device-approval:" + p.AccountID) {
			problem(w, 429, "rate_limited")
			return
		}
		x := h.registry.ApproveDeviceCode(r.Context(), p, str("userCode"))
		respond(w, map[string]bool{"ok": x == nil}, x)
	case path == "/device-codes/deny" && r.Method == "POST":
		if h.limited("device-approval:" + p.AccountID) {
			problem(w, 429, "rate_limited")
			return
		}
		x := h.registry.DenyDeviceCode(r.Context(), p, str("userCode"))
		respond(w, map[string]bool{"ok": x == nil}, x)
	default:
		problem(w, 404, "not_found")
	}
}
func (h *Handler) respondTokens(w http.ResponseWriter, r *http.Request, v account.Tokens, e error) {
	if e != nil {
		respond(w, nil, e)
		return
	}
	if r.Header.Get("X-VyNode-Client") != "native" {
		http.SetCookie(w, &http.Cookie{Name: "vynode_connect_refresh", Value: v.RefreshToken, Path: "/api/v1/account", HttpOnly: true, Secure: r.TLS != nil, SameSite: http.SameSiteStrictMode, MaxAge: int(h.accounts.RefreshTTL.Seconds())})
		v.RefreshToken = ""
	}
	respond(w, v, nil)
}
func respond(w http.ResponseWriter, v any, e error) {
	if e == nil {
		write(w, 200, v)
		return
	}
	switch {
	case errors.Is(e, account.ErrValidation), errors.Is(e, registry.ErrInvalid):
		problem(w, 400, "invalid_request")
	case errors.Is(e, account.ErrUnauthorized), errors.Is(e, account.ErrRevoked), errors.Is(e, registry.ErrForbidden):
		problem(w, 401, "authentication_required")
	case errors.Is(e, account.ErrConflict), errors.Is(e, registry.ErrConflict):
		problem(w, 409, "conflict")
	case errors.Is(e, registry.ErrGone):
		problem(w, 410, "gone")
	default:
		slog.Error("connect request failed", "error", e)
		problem(w, 500, "internal_error")
	}
}
func problem(w http.ResponseWriter, status int, code string) {
	write(w, status, map[string]any{"type": "https://vynode.invalid/problems/" + code, "title": http.StatusText(status), "status": status, "code": code})
}
func write(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}
