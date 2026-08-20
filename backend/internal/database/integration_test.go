//go:build integration

package database

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/thiagomontozo/restoreguard/backend/internal/security"
)

func integrationStore(t *testing.T) *Store {
	t.Helper()
	url := os.Getenv("RESTOREGUARD_TEST_DATABASE_URL")
	if url == "" {
		t.Fatal("RESTOREGUARD_TEST_DATABASE_URL is required")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	pool, err := pgxpool.New(ctx, url)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	if err = Migrate(ctx, pool, "../../../migrations"); err != nil {
		t.Fatal(err)
	}
	return NewStore(pool)
}
func createPrincipal(t *testing.T, s *Store, slug string) Principal {
	t.Helper()
	ctx := context.Background()
	hash, err := security.HashPassword("synthetic-password-for-tests")
	if err != nil {
		t.Fatal(err)
	}
	var p Principal
	if err = s.Pool.QueryRow(ctx, "INSERT INTO organizations(name,slug) VALUES($1,$2) RETURNING id", slug, slug).Scan(&p.OrganizationID); err != nil {
		t.Fatal(err)
	}
	if err = s.Pool.QueryRow(ctx, "INSERT INTO users(organization_id,email,password_hash,display_name) VALUES($1,$2,$3,'Test User') RETURNING id", p.OrganizationID, slug+"@example.test", hash).Scan(&p.UserID); err != nil {
		t.Fatal(err)
	}
	p.Email = slug + "@example.test"
	p.DisplayName = "Test User"
	p.Roles = []string{"OWNER"}
	return p
}
func TestMigrationsOrganizationIsolationAndIdempotency(t *testing.T) {
	s := integrationStore(t)
	suffix := uuid.NewString()[:8]
	a := createPrincipal(t, s, "org-a-"+suffix)
	b := createPrincipal(t, s, "org-b-"+suffix)
	assetA, err := s.CreateAsset(context.Background(), a.OrganizationID, Asset{Name: "ERP A " + suffix, Type: "POSTGRESQL_DATABASE", Environment: "TEST", Criticality: "CRITICAL"})
	if err != nil {
		t.Fatal(err)
	}
	assetB, err := s.CreateAsset(context.Background(), b.OrganizationID, Asset{Name: "ERP B " + suffix, Type: "POSTGRESQL_DATABASE", Environment: "TEST", Criticality: "HIGH"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err = s.GetAsset(context.Background(), a.OrganizationID, assetB.ID); err == nil {
		t.Fatal("organization A accessed organization B asset")
	}
	items, err := s.ListAssets(context.Background(), a.OrganizationID, "", 1, 100)
	if err != nil {
		t.Fatal(err)
	}
	for _, item := range items {
		if item.ID == assetB.ID {
			t.Fatal("cross-organization row leaked")
		}
	}
	drill1, created, err := s.CreateDrill(context.Background(), a, assetA.ID, "", "", "same-request-"+suffix)
	if err != nil || !created {
		t.Fatalf("create drill: %v %v", created, err)
	}
	drill2, created, err := s.CreateDrill(context.Background(), a, assetA.ID, "", "", "same-request-"+suffix)
	if err != nil || created || drill1.ID != drill2.ID {
		t.Fatalf("idempotency failed: %s %s %v %v", drill1.ID, drill2.ID, created, err)
	}
}
