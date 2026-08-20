//go:build integration

package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/thiagomontozo/restoreguard/backend/internal/database"
	"github.com/thiagomontozo/restoreguard/backend/internal/drill"
	"github.com/thiagomontozo/restoreguard/backend/internal/platform"
	"github.com/thiagomontozo/restoreguard/backend/internal/security"
)

type loginResult struct {
	CSRFToken string `json:"csrfToken"`
}

func loginForTest(t *testing.T, client *http.Client, base, email, password string) (*http.Cookie, string) {
	t.Helper()
	body, _ := json.Marshal(map[string]string{"email": email, "password": password})
	response, err := client.Post(base+"/api/v1/auth/login", "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		data, _ := io.ReadAll(response.Body)
		t.Fatalf("login status=%d %s", response.StatusCode, data)
	}
	var result loginResult
	if err = json.NewDecoder(response.Body).Decode(&result); err != nil {
		t.Fatal(err)
	}
	for _, cookie := range response.Cookies() {
		if cookie.Name == sessionCookie {
			return cookie, result.CSRFToken
		}
	}
	t.Fatal("session cookie missing")
	return nil, ""
}
func requestForTest(t *testing.T, client *http.Client, method, url string, body []byte, cookie *http.Cookie, csrf string) *http.Response {
	t.Helper()
	request, err := http.NewRequest(method, url, bytes.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Origin", "http://test.local")
	if cookie != nil {
		request.AddCookie(cookie)
	}
	if csrf != "" {
		request.Header.Set("X-CSRF-Token", csrf)
	}
	response, err := client.Do(request)
	if err != nil {
		t.Fatal(err)
	}
	return response
}
func TestAPIAuthenticationAuthorizationValidationAndIsolation(t *testing.T) {
	url := os.Getenv("RESTOREGUARD_TEST_DATABASE_URL")
	if url == "" {
		t.Fatal("RESTOREGUARD_TEST_DATABASE_URL is required")
	}
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, url)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	if err = database.Migrate(ctx, pool, "../../../migrations"); err != nil {
		t.Fatal(err)
	}
	suffix := uuid.NewString()[:8]
	var orgID string
	if err = pool.QueryRow(ctx, "INSERT INTO organizations(name,slug) VALUES($1,$2) RETURNING id", "API Test "+suffix, "api-test-"+suffix).Scan(&orgID); err != nil {
		t.Fatal(err)
	}
	ownerPassword := "owner-password-" + suffix
	viewerPassword := "viewer-password-" + suffix
	ownerHash, _ := security.HashPassword(ownerPassword)
	viewerHash, _ := security.HashPassword(viewerPassword)
	var ownerID, viewerID string
	if err = pool.QueryRow(ctx, "INSERT INTO users(organization_id,email,password_hash,display_name) VALUES($1,$2,$3,'Owner') RETURNING id", orgID, "owner-"+suffix+"@example.test", ownerHash).Scan(&ownerID); err != nil {
		t.Fatal(err)
	}
	if err = pool.QueryRow(ctx, "INSERT INTO users(organization_id,email,password_hash,display_name) VALUES($1,$2,$3,'Viewer') RETURNING id", orgID, "viewer-"+suffix+"@example.test", viewerHash).Scan(&viewerID); err != nil {
		t.Fatal(err)
	}
	if _, err = pool.Exec(ctx, "INSERT INTO user_roles(organization_id,user_id,role_id) SELECT $1,$2,id FROM roles WHERE name='OWNER'", orgID, ownerID); err != nil {
		t.Fatal(err)
	}
	if _, err = pool.Exec(ctx, "INSERT INTO user_roles(organization_id,user_id,role_id) SELECT $1,$2,id FROM roles WHERE name='VIEWER'", orgID, viewerID); err != nil {
		t.Fatal(err)
	}
	store := database.NewStore(pool)
	hub := drill.NewEventHub()
	workers := drill.NewWorkerPool(pool, platform.RealClock{}, hub, 1)
	defer func() {
		shutdown, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = workers.Close(shutdown)
		hub.Close()
	}()
	server := httptest.NewServer(New(store, workers, hub, slog.New(slog.NewTextHandler(io.Discard, nil)), false, "http://test.local"))
	defer server.Close()
	client := server.Client()
	response := requestForTest(t, client, http.MethodGet, server.URL+"/api/v1/protected-assets", nil, nil, "")
	if response.StatusCode != http.StatusUnauthorized {
		t.Fatalf("expected unauthorized, got %d", response.StatusCode)
	}
	response.Body.Close()
	ownerCookie, ownerCSRF := loginForTest(t, client, server.URL, "owner-"+suffix+"@example.test", ownerPassword)
	valid, _ := json.Marshal(map[string]string{"name": "Synthetic ERP", "type": "POSTGRESQL_DATABASE", "environment": "TEST", "criticality": "CRITICAL", "description": "Synthetic only"})
	response = requestForTest(t, client, http.MethodPost, server.URL+"/api/v1/protected-assets", valid, ownerCookie, ownerCSRF)
	if response.StatusCode != http.StatusCreated {
		data, _ := io.ReadAll(response.Body)
		t.Fatalf("expected created, got %d %s", response.StatusCode, data)
	}
	response.Body.Close()
	invalid, _ := json.Marshal(map[string]string{"name": "!", "type": "POSTGRESQL_DATABASE", "environment": "TEST", "criticality": "CRITICAL"})
	response = requestForTest(t, client, http.MethodPost, server.URL+"/api/v1/protected-assets", invalid, ownerCookie, ownerCSRF)
	if response.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected validation error, got %d", response.StatusCode)
	}
	response.Body.Close()
	response = requestForTest(t, client, http.MethodGet, server.URL+"/api/v1/protected-assets/"+uuid.NewString(), nil, ownerCookie, "")
	if response.StatusCode != http.StatusNotFound {
		t.Fatalf("expected not found, got %d", response.StatusCode)
	}
	response.Body.Close()
	viewerCookie, viewerCSRF := loginForTest(t, client, server.URL, "viewer-"+suffix+"@example.test", viewerPassword)
	response = requestForTest(t, client, http.MethodPost, server.URL+"/api/v1/protected-assets", valid, viewerCookie, viewerCSRF)
	if response.StatusCode != http.StatusForbidden {
		t.Fatalf("expected forbidden, got %d", response.StatusCode)
	}
	response.Body.Close()
}
