package httpserver

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"github.com/vynode/media/server/internal/auth"
	"github.com/vynode/media/server/internal/curation"
	"github.com/vynode/media/server/internal/intelligence"
	"github.com/vynode/media/server/internal/media"
	"github.com/vynode/media/server/internal/metadata"
	"github.com/vynode/media/server/internal/playback"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"runtime/debug"
	"time"
)

const apiVersion = "v1"

type contextKey string

const requestIDKey contextKey = "request_id"

type Readiness interface{ Ready(context.Context) error }
type SystemInfo struct {
	Version, Commit, OS, Architecture, InstanceID, ServerName, DatabaseType, WebDir string
	StartedAt                                                                       time.Time
}
type Handler struct {
	logger        *slog.Logger
	readiness     Readiness
	info          SystemInfo
	auth          *auth.Service
	media         *media.Service
	metadata      *metadata.Service
	playback      *playback.Service
	intelligence  *intelligence.Service
	curation      *curation.Service
	allowedOrigin string
}
type errorResponse struct {
	Error apiError `json:"error"`
}
type apiError struct {
	Code      string `json:"code"`
	Message   string `json:"message"`
	RequestID string `json:"requestId,omitempty"`
}

func NewHandler(logger *slog.Logger, readiness Readiness, info SystemInfo, authService *auth.Service, mediaService *media.Service, metadataService *metadata.Service, playbackService *playback.Service, allowedOrigin string, optional ...any) http.Handler {
	h := &Handler{logger: logger, readiness: readiness, info: info, auth: authService, media: mediaService, metadata: metadataService, playback: playbackService, allowedOrigin: allowedOrigin}
	for _, service := range optional {
		switch x := service.(type) {
		case *intelligence.Service:
			h.intelligence = x
		case *curation.Service:
			h.curation = x
		}
	}
	mux := http.NewServeMux()
	mux.HandleFunc("GET /health", h.health)
	mux.HandleFunc("GET /ready", h.ready)
	mux.HandleFunc("GET /api/v1/system/version", h.version)
	mux.HandleFunc("GET /api/v1/system/info", h.systemInfo)
	if authService != nil {
		h.authRoutes(mux)
	}
	if mediaService != nil {
		h.mediaRoutes(mux)
	}
	if metadataService != nil {
		h.metadataRoutes(mux)
	}
	if playbackService != nil {
		h.playbackRoutes(mux)
	}
	if h.intelligence != nil {
		h.intelligenceRoutes(mux)
	}
	if h.curation != nil {
		h.curationRoutes(mux)
	}
	if info.WebDir != "" {
		mux.Handle("/", spaHandler(info.WebDir))
	} else {
		mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
			writeError(w, r, http.StatusNotFound, "not_found", "The requested resource was not found.")
		})
	}
	return requestID(recoverer(logger, requestLog(logger, securityHeaders(h.originGuard(mux)))))
}

func spaHandler(webDir string) http.Handler {
	files := http.FileServer(http.Dir(webDir))
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet && r.Method != http.MethodHead {
			http.NotFound(w, r)
			return
		}
		path := filepath.Join(webDir, filepath.FromSlash(filepath.Clean("/"+r.URL.Path)))
		if stat, err := os.Stat(path); err == nil && !stat.IsDir() {
			files.ServeHTTP(w, r)
			return
		}
		http.ServeFile(w, r, filepath.Join(webDir, "index.html"))
	})
}
func (h *Handler) health(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}
func (h *Handler) ready(w http.ResponseWriter, r *http.Request) {
	if err := h.readiness.Ready(r.Context()); err != nil {
		writeError(w, r, http.StatusServiceUnavailable, "not_ready", "The server is not ready.")
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ready"})
}
func (h *Handler) version(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"version": h.info.Version, "commit": h.info.Commit, "apiVersion": apiVersion})
}
func (h *Handler) systemInfo(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{"version": h.info.Version, "operatingSystem": h.info.OS, "architecture": h.info.Architecture, "instanceId": h.info.InstanceID, "serverName": h.info.ServerName, "databaseType": h.info.DatabaseType, "uptimeSeconds": int64(time.Since(h.info.StartedAt).Seconds())})
}
func writeError(w http.ResponseWriter, r *http.Request, status int, code, message string) {
	writeJSON(w, status, errorResponse{Error: apiError{Code: code, Message: message, RequestID: RequestID(r.Context())}})
}
func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}
func RequestID(ctx context.Context) string {
	value, _ := ctx.Value(requestIDKey).(string)
	return value
}
func requestID(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id := r.Header.Get("X-Request-ID")
		if len(id) < 8 || len(id) > 128 {
			bytes := make([]byte, 16)
			_, _ = rand.Read(bytes)
			id = hex.EncodeToString(bytes)
		}
		w.Header().Set("X-Request-ID", id)
		next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), requestIDKey, id)))
	})
}
func securityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("X-Frame-Options", "DENY")
		w.Header().Set("Referrer-Policy", "no-referrer")
		w.Header().Set("Content-Security-Policy", "default-src 'self'; img-src 'self' data: blob:; media-src 'self' blob:; style-src 'self' 'unsafe-inline'; script-src 'self'; connect-src 'self'")
		w.Header().Set("Permissions-Policy", "camera=(), microphone=(), geolocation=()")
		next.ServeHTTP(w, r)
	})
}
func requestLog(logger *slog.Logger, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		started := time.Now()
		next.ServeHTTP(w, r)
		logger.Info("http request", "request_id", RequestID(r.Context()), "method", r.Method, "path", r.URL.Path, "duration_ms", time.Since(started).Milliseconds())
	})
}
func recoverer(logger *slog.Logger, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if recovered := recover(); recovered != nil {
				logger.Error("panic recovered", "request_id", RequestID(r.Context()), "value", recovered, "stack", string(debug.Stack()))
				writeError(w, r, http.StatusInternalServerError, "internal_error", "An unexpected error occurred.")
			}
		}()
		next.ServeHTTP(w, r)
	})
}
