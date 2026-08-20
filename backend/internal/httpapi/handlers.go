package httpapi

import (
	"context"
	"crypto/sha256"
	"crypto/subtle"
	"fmt"
	"net"
	"net/http"
	"regexp"
	"strings"
	"time"

	"github.com/thiagomontozo/restoreguard/backend/internal/database"
	reportgen "github.com/thiagomontozo/restoreguard/backend/internal/report"
)

var validName = regexp.MustCompile(`^[\pL\pN][\pL\pN ._:/()'-]{1,159}$`)

func timeContext(r *http.Request, timeout time.Duration) (context.Context, context.CancelFunc) {
	return context.WithTimeout(r.Context(), timeout)
}
func remoteIP(r *http.Request) net.IP {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return nil
	}
	return net.ParseIP(host)
}

func (s *Server) login(w http.ResponseWriter, r *http.Request) {
	var input struct {
		Email    string `json:"email"`
		Password string `json:"password"`
	}
	if decode(r, &input) != nil {
		writeError(w, r, http.StatusBadRequest, "VALIDATION_ERROR", "A valid email and password are required.")
		return
	}
	principal, token, csrf, err := s.store.Login(r.Context(), strings.TrimSpace(input.Email), input.Password, 12*time.Hour)
	if err != nil {
		writeError(w, r, http.StatusUnauthorized, "INVALID_CREDENTIALS", "Email or password is invalid.")
		return
	}
	http.SetCookie(w, &http.Cookie{Name: sessionCookie, Value: token, Path: "/", HttpOnly: true, Secure: s.cookieSecure, SameSite: http.SameSiteStrictMode, MaxAge: 43200})
	http.SetCookie(w, &http.Cookie{Name: csrfCookie, Value: csrf, Path: "/", HttpOnly: false, Secure: s.cookieSecure, SameSite: http.SameSiteStrictMode, MaxAge: 43200})
	_ = s.store.Audit(r.Context(), principal, "auth.login", "session", "", "{}", remoteIP(r))
	writeJSON(w, http.StatusOK, map[string]any{"user": principal, "csrfToken": csrf, "expiresIn": 43200})
}
func (s *Server) logout(w http.ResponseWriter, r *http.Request) {
	principal, _ := principalFrom(r.Context())
	cookie, _ := r.Cookie(sessionCookie)
	if cookie != nil {
		_ = s.store.Revoke(r.Context(), cookie.Value)
	}
	http.SetCookie(w, &http.Cookie{Name: sessionCookie, Value: "", Path: "/", HttpOnly: true, Secure: s.cookieSecure, SameSite: http.SameSiteStrictMode, MaxAge: -1})
	http.SetCookie(w, &http.Cookie{Name: csrfCookie, Value: "", Path: "/", Secure: s.cookieSecure, SameSite: http.SameSiteStrictMode, MaxAge: -1})
	_ = s.store.Audit(r.Context(), principal, "auth.logout", "session", "", "{}", remoteIP(r))
	w.WriteHeader(http.StatusNoContent)
}
func (s *Server) session(w http.ResponseWriter, r *http.Request) {
	principal, _ := principalFrom(r.Context())
	writeJSON(w, http.StatusOK, map[string]any{"user": principal})
}
func (s *Server) password(w http.ResponseWriter, r *http.Request) {
	principal, _ := principalFrom(r.Context())
	var input struct {
		CurrentPassword string `json:"currentPassword"`
		NewPassword     string `json:"newPassword"`
	}
	if decode(r, &input) != nil {
		writeError(w, r, http.StatusBadRequest, "VALIDATION_ERROR", "Password request is invalid.")
		return
	}
	if err := s.store.ChangePassword(r.Context(), principal, input.CurrentPassword, input.NewPassword); err != nil {
		writeError(w, r, http.StatusBadRequest, "PASSWORD_CHANGE_FAILED", err.Error())
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
func (s *Server) overview(w http.ResponseWriter, r *http.Request) {
	principal, _ := principalFrom(r.Context())
	value, err := s.store.Overview(r.Context(), principal.OrganizationID)
	if err != nil {
		dbError(w, r, err)
		return
	}
	protected := value["protectedAssets"]
	coverage := 0
	if protected > 0 {
		coverage = value["verifiedAssets"] * 100 / protected
	}
	value["recoveryCoveragePercent"] = coverage
	writeJSON(w, http.StatusOK, map[string]any{"data": value, "coverageExplanation": "Assets with a VERIFIED drill divided by enabled protected assets; this is not a safety guarantee."})
}
func (s *Server) assets(w http.ResponseWriter, r *http.Request) {
	principal, _ := principalFrom(r.Context())
	if r.Method == http.MethodGet {
		page, limit := pagination(r)
		items, err := s.store.ListAssets(r.Context(), principal.OrganizationID, r.URL.Query().Get("search"), page, limit)
		if err != nil {
			dbError(w, r, err)
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"data": items, "page": page, "limit": limit})
		return
	}
	var item database.Asset
	if decode(r, &item) != nil || !validName.MatchString(item.Name) {
		writeError(w, r, http.StatusBadRequest, "VALIDATION_ERROR", "Protected asset input is invalid.")
		return
	}
	allowedType := map[string]bool{"POSTGRESQL_DATABASE": true, "FILESYSTEM": true, "APPLICATION_DATASET": true}
	allowedCriticality := map[string]bool{"LOW": true, "MEDIUM": true, "HIGH": true, "CRITICAL": true}
	if !allowedType[item.Type] || !allowedCriticality[item.Criticality] {
		writeError(w, r, http.StatusBadRequest, "VALIDATION_ERROR", "Asset type or criticality is invalid.")
		return
	}
	created, err := s.store.CreateAsset(r.Context(), principal.OrganizationID, item)
	if err != nil {
		dbError(w, r, err)
		return
	}
	_ = s.store.Audit(r.Context(), principal, "protected_asset.created", "protected_asset", created.ID, "{}", remoteIP(r))
	writeJSON(w, http.StatusCreated, created)
}
func (s *Server) asset(w http.ResponseWriter, r *http.Request) {
	principal, _ := principalFrom(r.Context())
	item, err := s.store.GetAsset(r.Context(), principal.OrganizationID, r.PathValue("id"))
	if err != nil {
		dbError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, item)
}
func (s *Server) sources(w http.ResponseWriter, r *http.Request) {
	principal, _ := principalFrom(r.Context())
	if r.Method == http.MethodGet {
		page, limit := pagination(r)
		items, err := s.store.ListSources(r.Context(), principal.OrganizationID, page, limit)
		if err != nil {
			dbError(w, r, err)
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"data": items, "page": page, "limit": limit})
		return
	}
	var item database.Source
	if decode(r, &item) != nil || !validName.MatchString(item.Name) {
		writeError(w, r, http.StatusBadRequest, "VALIDATION_ERROR", "Backup source input is invalid.")
		return
	}
	allowed := map[string]bool{"LOCAL_FILESYSTEM": true, "S3_COMPATIBLE": true, "POSTGRES_DUMP": true}
	if !allowed[item.Type] {
		writeError(w, r, http.StatusBadRequest, "VALIDATION_ERROR", "Backup source type is invalid.")
		return
	}
	created, err := s.store.CreateSource(r.Context(), principal.OrganizationID, item)
	if err != nil {
		dbError(w, r, err)
		return
	}
	_ = s.store.Audit(r.Context(), principal, "backup_source.created", "backup_source", created.ID, "{}", remoteIP(r))
	writeJSON(w, http.StatusCreated, created)
}
func (s *Server) discover(w http.ResponseWriter, r *http.Request) {
	principal, _ := principalFrom(r.Context())
	if err := s.store.DiscoverSource(r.Context(), principal.OrganizationID, r.PathValue("id")); err != nil {
		dbError(w, r, err)
		return
	}
	_ = s.store.Audit(r.Context(), principal, "backup_source.discovery_completed", "backup_source", r.PathValue("id"), "{\"status\":\"SUCCEEDED\"}", remoteIP(r))
	writeJSON(w, http.StatusAccepted, map[string]string{"status": "SUCCEEDED"})
}
func (s *Server) policies(w http.ResponseWriter, r *http.Request) {
	principal, _ := principalFrom(r.Context())
	if r.Method == http.MethodGet {
		page, limit := pagination(r)
		items, err := s.store.ListPolicies(r.Context(), principal.OrganizationID, page, limit)
		if err != nil {
			dbError(w, r, err)
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"data": items, "page": page, "limit": limit})
		return
	}
	var item database.Policy
	if decode(r, &item) != nil || !validName.MatchString(item.Name) || !map[string]bool{"DAILY": true, "WEEKLY": true, "MONTHLY": true}[item.Schedule] {
		writeError(w, r, http.StatusBadRequest, "VALIDATION_ERROR", "Recovery policy input is invalid.")
		return
	}
	created, err := s.store.CreatePolicy(r.Context(), principal.OrganizationID, item)
	if err != nil {
		writeError(w, r, http.StatusBadRequest, "VALIDATION_ERROR", err.Error())
		return
	}
	_ = s.store.Audit(r.Context(), principal, "policy.created", "recovery_policy", created.ID, "{}", remoteIP(r))
	writeJSON(w, http.StatusCreated, created)
}
func (s *Server) drills(w http.ResponseWriter, r *http.Request) {
	principal, _ := principalFrom(r.Context())
	if r.Method == http.MethodGet {
		page, limit := pagination(r)
		items, err := s.store.ListDrills(r.Context(), principal.OrganizationID, r.URL.Query().Get("status"), r.URL.Query().Get("asset"), page, limit)
		if err != nil {
			dbError(w, r, err)
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"data": items, "page": page, "limit": limit})
		return
	}
	var input struct {
		ProtectedAssetID string `json:"protectedAssetId"`
		BackupSnapshotID string `json:"backupSnapshotId"`
		RecoveryPolicyID string `json:"recoveryPolicyId"`
	}
	if decode(r, &input) != nil || input.ProtectedAssetID == "" {
		writeError(w, r, http.StatusBadRequest, "VALIDATION_ERROR", "A protected asset is required.")
		return
	}
	item, created, err := s.store.CreateDrill(r.Context(), principal, input.ProtectedAssetID, input.BackupSnapshotID, input.RecoveryPolicyID, r.Header.Get("Idempotency-Key"))
	if err != nil {
		dbError(w, r, err)
		return
	}
	if created {
		_ = s.store.Audit(r.Context(), principal, "drill.created", "recovery_drill", item.ID, "{}", remoteIP(r))
		if err := s.workers.Submit(item.ID); err != nil {
			writeError(w, r, http.StatusServiceUnavailable, "DRILL_QUEUE_FULL", "The drill queue is currently full.")
			return
		}
	}
	writeJSON(w, http.StatusAccepted, map[string]any{"data": item, "created": created})
}
func (s *Server) drill(w http.ResponseWriter, r *http.Request) {
	principal, _ := principalFrom(r.Context())
	item, err := s.store.GetDrill(r.Context(), principal.OrganizationID, r.PathValue("id"))
	if err != nil {
		dbError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, item)
}
func (s *Server) cancel(w http.ResponseWriter, r *http.Request) {
	principal, _ := principalFrom(r.Context())
	id := r.PathValue("id")
	s.workers.Cancel(id)
	if err := s.store.CancelDrill(r.Context(), principal.OrganizationID, id); err != nil {
		dbError(w, r, err)
		return
	}
	_ = s.store.Audit(r.Context(), principal, "drill.cancelled", "recovery_drill", id, "{}", remoteIP(r))
	writeJSON(w, http.StatusOK, map[string]string{"status": "CANCELLED"})
}
func (s *Server) events(w http.ResponseWriter, r *http.Request) {
	principal, _ := principalFrom(r.Context())
	id := r.PathValue("id")
	if _, err := s.store.GetDrill(r.Context(), principal.OrganizationID, id); err != nil {
		dbError(w, r, err)
		return
	}
	flusher, ok := w.(http.Flusher)
	if !ok {
		writeError(w, r, http.StatusInternalServerError, "SSE_UNAVAILABLE", "Streaming is unavailable.")
		return
	}
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("X-Accel-Buffering", "no")
	events, unsubscribe := s.hub.Subscribe(id)
	defer unsubscribe()
	fmt.Fprint(w, "event: connected\ndata: {}\n\n")
	flusher.Flush()
	heartbeat := time.NewTicker(15 * time.Second)
	defer heartbeat.Stop()
	for {
		select {
		case <-r.Context().Done():
			return
		case payload, ok := <-events:
			if !ok {
				return
			}
			fmt.Fprintf(w, "event: progress\ndata: %s\n\n", payload)
			flusher.Flush()
		case <-heartbeat.C:
			fmt.Fprint(w, ": keepalive\n\n")
			flusher.Flush()
		}
	}
}
func (s *Server) report(w http.ResponseWriter, r *http.Request) {
	principal, _ := principalFrom(r.Context())
	item, err := s.store.GetDrill(r.Context(), principal.OrganizationID, r.PathValue("id"))
	if err != nil {
		dbError(w, r, err)
		return
	}
	pdf := reportgen.GeneratePDF(reportgen.RecoveryReport{Asset: item.ProtectedAssetID, Snapshot: item.BackupSnapshotID, Sandbox: "Docker isolated restore sandbox (destroyed after drill)", RPO: seconds(item.MeasuredRPOSeconds), RTO: seconds(item.MeasuredRTOSeconds), RPOResult: string(item.RPOResult), RTOResult: string(item.RTOResult), Assessment: string(item.RecoveryStatus), Confidence: string(item.Confidence), GeneratedAt: time.Now().UTC(), Validations: []string{"Typed required checks"}, Evidence: []string{"SHA-256 integrity metadata"}, Limitations: "Controlled conditions; PostgreSQL/Docker workflow only in v0.1."})
	w.Header().Set("Content-Type", "application/pdf")
	w.Header().Set("Content-Disposition", "attachment; filename=recovery-verification-report.pdf")
	w.Header().Set("Cache-Control", "private, no-store")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(pdf)
}
func seconds(value *int64) string {
	if value == nil {
		return "INCONCLUSIVE"
	}
	return (time.Duration(*value) * time.Second).String()
}
func (s *Server) snapshots(w http.ResponseWriter, r *http.Request) {
	principal, _ := principalFrom(r.Context())
	rows, err := s.store.Pool.Query(r.Context(), "SELECT id,backup_source_id,external_id,name,type,completed_at,size_bytes,checksum,status,discovered_at,last_verified_at FROM backup_snapshots WHERE organization_id=$1 AND ($2='' OR backup_source_id::text=$2) AND ($3='' OR status=$3) ORDER BY completed_at DESC NULLS LAST LIMIT 100", principal.OrganizationID, r.URL.Query().Get("source"), r.URL.Query().Get("status"))
	if err != nil {
		dbError(w, r, err)
		return
	}
	defer rows.Close()
	data := []map[string]any{}
	for rows.Next() {
		var id, sourceID, externalID, name, kind, status string
		var completedAt, lastVerifiedAt *time.Time
		var discoveredAt time.Time
		var size int64
		var checksum *string
		if err := rows.Scan(&id, &sourceID, &externalID, &name, &kind, &completedAt, &size, &checksum, &status, &discoveredAt, &lastVerifiedAt); err != nil {
			dbError(w, r, err)
			return
		}
		data = append(data, map[string]any{"id": id, "backupSourceId": sourceID, "externalId": externalID, "name": name, "type": kind, "completedAt": completedAt, "sizeBytes": size, "checksum": checksum, "status": status, "discoveredAt": discoveredAt, "lastVerifiedAt": lastVerifiedAt})
	}
	writeJSON(w, http.StatusOK, map[string]any{"data": data})
}
func (s *Server) evidence(w http.ResponseWriter, r *http.Request) {
	s.simpleRows(w, r, "SELECT id,drill_id,type,summary,sha256,created_at FROM evidence WHERE organization_id=$1 ORDER BY created_at DESC LIMIT 100", []string{"id", "drillId", "type", "summary", "sha256", "createdAt"})
}
func (s *Server) audit(w http.ResponseWriter, r *http.Request) {
	s.simpleRows(w, r, "SELECT id,event_type,resource_type,resource_id,metadata,timestamp FROM audit_events WHERE organization_id=$1 ORDER BY timestamp DESC LIMIT 100", []string{"id", "eventType", "resourceType", "resourceId", "metadata", "timestamp"})
}
func (s *Server) simpleRows(w http.ResponseWriter, r *http.Request, query string, keys []string) {
	principal, _ := principalFrom(r.Context())
	rows, err := s.store.Pool.Query(r.Context(), query, principal.OrganizationID)
	if err != nil {
		dbError(w, r, err)
		return
	}
	defer rows.Close()
	data := []map[string]any{}
	for rows.Next() {
		values, err := rows.Values()
		if err != nil {
			dbError(w, r, err)
			return
		}
		item := map[string]any{}
		for i, key := range keys {
			item[key] = values[i]
		}
		data = append(data, item)
	}
	writeJSON(w, http.StatusOK, map[string]any{"data": data})
}
func (s *Server) roles(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{"data": []string{"OWNER", "ADMIN", "RECOVERY_ENGINEER", "OPERATOR", "AUDITOR", "VIEWER"}})
}
func (s *Server) users(w http.ResponseWriter, r *http.Request) {
	principal, _ := principalFrom(r.Context())
	writeJSON(w, http.StatusOK, map[string]any{"data": []database.Principal{principal}})
}
func (s *Server) settings(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{"mfaFoundation": "TOTP-ready; enrollment is not implemented in v0.1", "status": "EXPERIMENTAL"})
}
func validateCSRF(cookieValue string, hash []byte) bool {
	sum := sha256.Sum256([]byte(cookieValue))
	return subtle.ConstantTimeCompare(sum[:], hash) == 1
}
