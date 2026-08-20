package httpapi

import (
	"context"
	"crypto/sha256"
	"crypto/subtle"
	"github.com/google/uuid"
	"github.com/thiagomontozo/restoreguard/backend/internal/database"
	"github.com/thiagomontozo/restoreguard/backend/internal/security"
	"net/http"
	"strings"
)

type contextKey string

const (
	principalKey  contextKey = "principal"
	requestIDKey  contextKey = "requestID"
	sessionCookie            = "restoreguard_session"
	csrfCookie               = "restoreguard_csrf"
)

func principalFrom(ctx context.Context) (database.Principal, bool) {
	value, ok := ctx.Value(principalKey).(database.Principal)
	return value, ok
}
func requestID(ctx context.Context) string {
	value, _ := ctx.Value(requestIDKey).(string)
	return value
}
func (s *Server) secure(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id := r.Header.Get("X-Request-ID")
		if _, err := uuid.Parse(id); err != nil {
			id = uuid.NewString()
		}
		w.Header().Set("X-Request-ID", id)
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("Referrer-Policy", "no-referrer")
		w.Header().Set("X-Frame-Options", "DENY")
		w.Header().Set("Content-Security-Policy", "default-src 'self'; frame-ancestors 'none'; base-uri 'none'")
		if origin := r.Header.Get("Origin"); origin != "" && origin == s.corsOrigin {
			w.Header().Set("Access-Control-Allow-Origin", origin)
			w.Header().Set("Access-Control-Allow-Credentials", "true")
			w.Header().Set("Vary", "Origin")
			if r.Method == http.MethodOptions {
				w.Header().Set("Access-Control-Allow-Headers", "Content-Type,X-CSRF-Token,X-Request-ID,Idempotency-Key")
				w.Header().Set("Access-Control-Allow-Methods", "GET,POST,PUT,PATCH,DELETE,OPTIONS")
				w.WriteHeader(http.StatusNoContent)
				return
			}
		}
		next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), requestIDKey, id)))
	})
}
func (s *Server) require(permission string, next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		cookie, err := r.Cookie(sessionCookie)
		if err != nil {
			writeError(w, r, http.StatusUnauthorized, "UNAUTHORIZED", "Authentication is required.")
			return
		}
		principal, csrfHash, err := s.store.Authenticate(r.Context(), cookie.Value)
		if err != nil {
			writeError(w, r, http.StatusUnauthorized, "UNAUTHORIZED", "Session is invalid or expired.")
			return
		}
		if permission != "" && !security.Allowed(principal.Roles, permission) {
			writeError(w, r, http.StatusForbidden, "FORBIDDEN", "You do not have permission to perform this action.")
			return
		}
		if r.Method != http.MethodGet && r.Method != http.MethodHead && r.Method != http.MethodOptions {
			token := r.Header.Get("X-CSRF-Token")
			sum := sha256.Sum256([]byte(token))
			if token == "" || subtle.ConstantTimeCompare(sum[:], csrfHash) != 1 {
				writeError(w, r, http.StatusForbidden, "CSRF_INVALID", "CSRF validation failed.")
				return
			}
			origin := r.Header.Get("Origin")
			if origin != "" && !strings.EqualFold(origin, s.corsOrigin) {
				writeError(w, r, http.StatusForbidden, "ORIGIN_INVALID", "Request origin is not allowed.")
				return
			}
		}
		next(w, r.WithContext(context.WithValue(r.Context(), principalKey, principal)))
	}
}
