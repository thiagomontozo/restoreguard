package notify

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"
)

type Event struct {
	OrganizationID, Type, ResourceID, Summary string
	Timestamp                                 time.Time
}
type Notifier interface {
	Notify(context.Context, Event) error
}
type InApp struct {
	Deliver func(context.Context, Event) error
}

func (n InApp) Notify(ctx context.Context, event Event) error { return n.Deliver(ctx, event) }

type Webhook struct {
	endpoint     *url.URL
	secret       []byte
	client       *http.Client
	allowedHosts map[string]bool
}

func NewWebhook(endpoint string, secret []byte, allowedHosts []string) (*Webhook, error) {
	parsed, err := url.Parse(endpoint)
	if err != nil || parsed.Scheme != "https" || parsed.User != nil || parsed.Hostname() == "" {
		return nil, errors.New("webhook must be an HTTPS URL without user information")
	}
	allowed := map[string]bool{}
	for _, host := range allowedHosts {
		allowed[strings.ToLower(host)] = true
	}
	if !allowed[strings.ToLower(parsed.Hostname())] {
		return nil, errors.New("webhook host is not allowlisted")
	}
	return &Webhook{endpoint: parsed, secret: secret, allowedHosts: allowed, client: &http.Client{Timeout: 5 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return errors.New("webhook redirects are disabled") }}}, nil
}
func (w *Webhook) Notify(ctx context.Context, event Event) error {
	addresses, err := net.DefaultResolver.LookupIPAddr(ctx, w.endpoint.Hostname())
	if err != nil {
		return err
	}
	for _, address := range addresses {
		if address.IP.IsPrivate() || address.IP.IsLoopback() || address.IP.IsLinkLocalUnicast() || address.IP.IsUnspecified() {
			return errors.New("webhook resolved to a restricted address")
		}
	}
	payload, err := json.Marshal(event)
	if err != nil {
		return err
	}
	if len(payload) > 64*1024 {
		return errors.New("webhook payload exceeds limit")
	}
	mac := hmac.New(sha256.New, w.secret)
	_, _ = mac.Write(payload)
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, w.endpoint.String(), bytes.NewReader(payload))
	if err != nil {
		return err
	}
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("X-RestoreGuard-Signature", "sha256="+hex.EncodeToString(mac.Sum(nil)))
	response, err := w.client.Do(request)
	if err != nil {
		return err
	}
	defer response.Body.Close()
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return fmt.Errorf("webhook returned %d", response.StatusCode)
	}
	return nil
}
