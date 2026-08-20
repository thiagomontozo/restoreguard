package notify

import "testing"

func TestWebhookRequiresHTTPSAndAllowlist(t *testing.T) {
	if _, err := NewWebhook("http://127.0.0.1/hook", []byte("secret"), []string{"127.0.0.1"}); err == nil {
		t.Fatal("HTTP webhook should fail")
	}
	if _, err := NewWebhook("https://example.com/hook", []byte("secret"), []string{"hooks.example.com"}); err == nil {
		t.Fatal("non-allowlisted host should fail")
	}
}
