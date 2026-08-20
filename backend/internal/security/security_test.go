package security

import (
	"encoding/base64"
	"strings"
	"testing"
)

func TestPasswordRoundTrip(t *testing.T) {
	hash, err := HashPassword("correct-horse-battery-staple")
	if err != nil {
		t.Fatal(err)
	}
	if !VerifyPassword(hash, "correct-horse-battery-staple") || VerifyPassword(hash, "wrong-password") {
		t.Fatal("password verification failed")
	}
}
func TestSecretStoreOrganizationBinding(t *testing.T) {
	key := base64.StdEncoding.EncodeToString([]byte(strings.Repeat("k", 32)))
	store, err := NewSecretStore(key)
	if err != nil {
		t.Fatal(err)
	}
	ciphertext, err := store.Encrypt("org-a", []byte("secret"))
	if err != nil {
		t.Fatal(err)
	}
	plain, err := store.Decrypt("org-a", ciphertext)
	if err != nil || string(plain) != "secret" {
		t.Fatal("roundtrip failed")
	}
	if _, err := store.Decrypt("org-b", ciphertext); err == nil {
		t.Fatal("cross-organization decryption should fail")
	}
}
func TestPermissions(t *testing.T) {
	if !Allowed([]string{"RECOVERY_ENGINEER"}, "drill.run") {
		t.Fatal("engineer should run drills")
	}
	if Allowed([]string{"VIEWER"}, "drill.run") {
		t.Fatal("viewer must not run drills")
	}
}
