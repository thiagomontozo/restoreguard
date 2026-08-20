package httpapi

import (
	"encoding/json"
	"errors"
	"github.com/jackc/pgx/v5"
	"github.com/thiagomontozo/restoreguard/backend/internal/database"
	"github.com/thiagomontozo/restoreguard/backend/internal/drill"
	"log/slog"
	"net/http"
	"strconv"
	"time"
)

type Server struct {
	store        *database.Store
	workers      *drill.WorkerPool
	hub          *drill.EventHub
	logger       *slog.Logger
	cookieSecure bool
	corsOrigin   string
}

func New(store *database.Store, workers *drill.WorkerPool, hub *drill.EventHub, logger *slog.Logger, cookieSecure bool, corsOrigin string) http.Handler {
	s := &Server{store: store, workers: workers, hub: hub, logger: logger, cookieSecure: cookieSecure, corsOrigin: corsOrigin}
	mux := http.NewServeMux()
	mux.HandleFunc("GET /health", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	})
	mux.HandleFunc("GET /ready", s.ready)
	mux.HandleFunc("POST /api/v1/auth/login", s.login)
	mux.HandleFunc("POST /api/v1/auth/logout", s.require("", s.logout))
	mux.HandleFunc("GET /api/v1/auth/session", s.require("", s.session))
	mux.HandleFunc("POST /api/v1/auth/password", s.require("", s.password))
	mux.HandleFunc("GET /api/v1/overview", s.require("drill.read", s.overview))
	mux.HandleFunc("GET /api/v1/protected-assets", s.require("drill.read", s.assets))
	mux.HandleFunc("POST /api/v1/protected-assets", s.require("settings.manage", s.assets))
	mux.HandleFunc("GET /api/v1/protected-assets/{id}", s.require("drill.read", s.asset))
	mux.HandleFunc("GET /api/v1/backup-sources", s.require("backup_source.read", s.sources))
	mux.HandleFunc("POST /api/v1/backup-sources", s.require("backup_source.manage", s.sources))
	mux.HandleFunc("POST /api/v1/backup-sources/{id}/discover", s.require("backup_source.manage", s.discover))
	mux.HandleFunc("GET /api/v1/recovery-policies", s.require("recovery_policy.read", s.policies))
	mux.HandleFunc("POST /api/v1/recovery-policies", s.require("recovery_policy.manage", s.policies))
	mux.HandleFunc("GET /api/v1/drills", s.require("drill.read", s.drills))
	mux.HandleFunc("POST /api/v1/drills", s.require("drill.run", s.drills))
	mux.HandleFunc("GET /api/v1/drills/{id}", s.require("drill.read", s.drill))
	mux.HandleFunc("POST /api/v1/drills/{id}/cancel", s.require("drill.run", s.cancel))
	mux.HandleFunc("GET /api/v1/drills/{id}/events", s.require("drill.read", s.events))
	mux.HandleFunc("GET /api/v1/drills/{id}/report", s.require("report.export", s.report))
	mux.HandleFunc("GET /api/v1/snapshots", s.require("backup_source.read", s.snapshots))
	mux.HandleFunc("GET /api/v1/evidence", s.require("evidence.read", s.evidence))
	mux.HandleFunc("GET /api/v1/audit", s.require("audit.read", s.audit))
	mux.HandleFunc("GET /api/v1/roles", s.require("user.manage", s.roles))
	mux.HandleFunc("GET /api/v1/users", s.require("user.manage", s.users))
	mux.HandleFunc("GET /api/v1/settings", s.require("settings.manage", s.settings))
	return s.secure(mux)
}
func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}
func writeError(w http.ResponseWriter, r *http.Request, status int, code, message string) {
	writeJSON(w, status, map[string]any{"error": map[string]string{"code": code, "message": message, "requestId": requestID(r.Context())}})
}
func decode(r *http.Request, value any) error {
	r.Body = http.MaxBytesReader(nil, r.Body, 1<<20)
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	return decoder.Decode(value)
}
func pagination(r *http.Request) (int, int) {
	page, _ := strconv.Atoi(r.URL.Query().Get("page"))
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	if page < 1 {
		page = 1
	}
	if limit < 1 {
		limit = 20
	}
	return page, limit
}
func (s *Server) ready(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := timeContext(r, 3*time.Second)
	defer cancel()
	if err := s.store.Health(ctx); err != nil {
		writeError(w, r, http.StatusServiceUnavailable, "NOT_READY", "Database is not ready.")
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ready"})
}
func dbError(w http.ResponseWriter, r *http.Request, err error) {
	if errors.Is(err, pgx.ErrNoRows) {
		writeError(w, r, http.StatusNotFound, "NOT_FOUND", "The requested resource was not found.")
		return
	}
	writeError(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", "The request could not be completed.")
}
