package platform

import (
	"context"
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func githubAppFixture(t *testing.T) (Connector, *rsa.PrivateKey) {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	keyPath := filepath.Join(dir, "app.pem")
	if err = os.WriteFile(keyPath, pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(key)}), 0600); err != nil {
		t.Fatal(err)
	}
	config := filepath.Join(dir, "apps.json")
	raw, _ := json.Marshal(map[string]any{"TEST_APP_BOT": map[string]any{"app_id": "123", "installation_id": 456, "private_key_file": keyPath}})
	if err = os.WriteFile(config, raw, 0600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PLATFORM_GITHUB_APP_CONFIG", config)
	t.Setenv("TEST_APP_BOT", "")
	original := githubClient
	t.Cleanup(func() { githubClient = original })
	return Connector{Kind: "github.issue_comment", Repository: "owner/repo", TokenEnv: "TEST_APP_BOT"}, key
}

func TestGitHubAppSignsScopedTokenAndReusesIt(t *testing.T) {
	v, key := githubAppFixture(t)
	minted, calls := 0, 0
	githubClient = &http.Client{Transport: connectorTransport(func(r *http.Request) (*http.Response, error) {
		if r.URL.Host != "api.github.com" {
			t.Fatal("credential sent outside GitHub")
		}
		body := `{}`
		if r.URL.Path == "/app/installations/456/access_tokens" {
			minted++
			parts := strings.Split(strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer "), ".")
			if len(parts) != 3 || r.Method != "POST" {
				t.Fatal("missing signed App authentication")
			}
			sig, _ := base64.RawURLEncoding.DecodeString(parts[2])
			hash := sha256.Sum256([]byte(parts[0] + "." + parts[1]))
			if err := rsa.VerifyPKCS1v15(&key.PublicKey, crypto.SHA256, hash[:], sig); err != nil {
				t.Fatal("invalid App signature")
			}
			claims, _ := base64.RawURLEncoding.DecodeString(parts[1])
			var payload struct {
				Issuer string `json:"iss"`
				Expiry int64  `json:"exp"`
			}
			if json.Unmarshal(claims, &payload) != nil || payload.Issuer != "123" || payload.Expiry <= time.Now().Unix() || payload.Expiry > time.Now().Add(10*time.Minute).Unix() {
				t.Fatal("invalid App claims")
			}
			var scope struct {
				Repositories []string `json:"repositories"`
			}
			if json.NewDecoder(r.Body).Decode(&scope) != nil || len(scope.Repositories) != 1 || scope.Repositories[0] != "repo" {
				t.Fatal("token not restricted to Connector repository")
			}
			body = fmt.Sprintf(`{"token":"installation-secret","expires_at":%q}`, time.Now().Add(time.Hour).UTC().Format(time.RFC3339))
		} else {
			calls++
			if r.Header.Get("Authorization") != "Bearer installation-secret" {
				t.Fatal("request not attributed to App installation")
			}
		}
		return &http.Response{StatusCode: 200, Header: http.Header{}, Body: io.NopCloser(strings.NewReader(body))}, nil
	})}
	for range 2 {
		if err := githubRequest(context.Background(), v, "GET", "/repos/owner/repo", nil, nil); err != nil {
			t.Fatal(err)
		}
	}
	if minted != 1 || calls != 2 {
		t.Fatal("usable token not reused", minted, calls)
	}
}

func TestGitHubAppRefreshesNearExpiry(t *testing.T) {
	v, _ := githubAppFixture(t)
	minted := 0
	githubClient = &http.Client{Transport: connectorTransport(func(r *http.Request) (*http.Response, error) {
		body := `{}`
		if r.URL.Path == "/app/installations/456/access_tokens" {
			minted++
			body = fmt.Sprintf(`{"token":"installation-secret-%d","expires_at":%q}`, minted, time.Now().Add(45*time.Second).UTC().Format(time.RFC3339))
		}
		return &http.Response{StatusCode: 200, Header: http.Header{}, Body: io.NopCloser(strings.NewReader(body))}, nil
	})}
	for range 2 {
		if err := githubRequest(context.Background(), v, "GET", "/repos/owner/repo", nil, nil); err != nil {
			t.Fatal(err)
		}
	}
	if minted != 2 {
		t.Fatal("near-expiry token reused", minted)
	}
}

func TestGitHubAppMissingKeyDoesNotFallback(t *testing.T) {
	v, _ := githubAppFixture(t)
	writes := 0
	githubClient = &http.Client{Transport: connectorTransport(func(r *http.Request) (*http.Response, error) {
		writes++
		return &http.Response{StatusCode: 200, Header: http.Header{}, Body: io.NopCloser(strings.NewReader(`{}`))}, nil
	})}
	// A configured App must not silently post as a personal account after key loss.
	t.Setenv("TEST_APP_BOT", "personal-token")
	if err := os.WriteFile(os.Getenv("PLATFORM_GITHUB_APP_CONFIG"), []byte(`{"TEST_APP_BOT":{"app_id":"123","installation_id":456,"private_key_file":"/missing.pem"}}`), 0600); err != nil {
		t.Fatal(err)
	}
	if err := githubRequest(context.Background(), v, "POST", "/repos/owner/repo/issues/1/comments", map[string]string{"body": "test"}, nil); err == nil {
		t.Fatal("invalid App credential fell back to personal identity")
	}
	if writes != 0 {
		t.Fatal("posted with a broken App credential")
	}
}

func TestGitHubAppNullRegistryDoesNotFallback(t *testing.T) {
	v, _ := githubAppFixture(t)
	t.Setenv("TEST_APP_BOT", "personal-token")
	if err := os.WriteFile(os.Getenv("PLATFORM_GITHUB_APP_CONFIG"), []byte(`null`), 0600); err != nil {
		t.Fatal(err)
	}
	githubClient = &http.Client{Transport: connectorTransport(func(r *http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: 200, Header: http.Header{}, Body: io.NopCloser(strings.NewReader(`{}`))}, nil
	})}
	if err := githubRequest(context.Background(), v, "POST", "/repos/owner/repo/issues/1/comments", map[string]string{"body": "test"}, nil); err == nil {
		t.Fatal("corrupt registry silently used personal identity")
	}
}

func TestGitHubAppPermissionFailureDoesNotPostOrExposeResponse(t *testing.T) {
	v, _ := githubAppFixture(t)
	calls := 0
	githubClient = &http.Client{Transport: connectorTransport(func(r *http.Request) (*http.Response, error) {
		calls++
		return &http.Response{StatusCode: 403, Header: http.Header{}, Body: io.NopCloser(strings.NewReader(`{"message":"private-token-value"}`))}, nil
	})}
	err := githubRequest(context.Background(), v, "POST", "/repos/owner/repo/issues/1/comments", map[string]string{"body": "test"}, nil)
	if err == nil || calls != 1 || strings.Contains(err.Error(), "private-token-value") {
		t.Fatal("permission failure retried, posted, or exposed the response")
	}
}

func TestGitHubAppCancellationWhileAnotherRequestMints(t *testing.T) {
	v, _ := githubAppFixture(t)
	started, release := make(chan struct{}), make(chan struct{})
	githubClient = &http.Client{Transport: connectorTransport(func(r *http.Request) (*http.Response, error) {
		body := `{}`
		if r.URL.Path == "/app/installations/456/access_tokens" {
			close(started)
			<-release
			body = fmt.Sprintf(`{"token":"installation-secret","expires_at":%q}`, time.Now().Add(time.Hour).UTC().Format(time.RFC3339))
		}
		return &http.Response{StatusCode: 200, Header: http.Header{}, Body: io.NopCloser(strings.NewReader(body))}, nil
	})}
	done := make(chan error, 1)
	go func() { done <- githubRequest(context.Background(), v, "GET", "/repos/owner/repo", nil, nil) }()
	<-started
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()
	err := githubRequest(ctx, v, "GET", "/repos/owner/repo", nil, nil)
	close(release)
	first := <-done
	if !errors.Is(err, context.DeadlineExceeded) || first != nil {
		t.Fatal("waiting for shared token did not cancel", err, first)
	}
}
