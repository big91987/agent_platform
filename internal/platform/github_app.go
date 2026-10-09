package platform

import (
	"bytes"
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
	"regexp"
	"strings"
	"sync"
	"time"
)

// The private server registry maps existing Connector credential names to Apps.
// App secrets and short-lived tokens never enter workflow snapshots or Agents.
type githubAppConfig struct {
	AppID          string `json:"app_id"`
	InstallationID int64  `json:"installation_id"`
	PrivateKeyFile string `json:"private_key_file"`
}

func githubPrivateFile(path string) ([]byte, error) {
	info, err := os.Lstat(path)
	if err != nil || !filepath.IsAbs(path) || !info.Mode().IsRegular() || info.Mode().Perm()&0077 != 0 || info.Size() > 65536 {
		return nil, errors.New("GitHub App configuration/key must be a bounded private regular file")
	}
	file, err := os.Open(path)
	if err != nil {
		return nil, errors.New("GitHub App private file cannot be read")
	}
	defer file.Close()
	raw, err := io.ReadAll(io.LimitReader(file, 65537))
	if err != nil || len(raw) > 65536 {
		return nil, errors.New("GitHub App private file cannot be read within size limit")
	}
	return raw, nil
}

func connectorGitHubApp(v Connector) (*githubAppConfig, *rsa.PrivateKey, [32]byte, error) {
	var fingerprint [32]byte
	path := os.Getenv("PLATFORM_GITHUB_APP_CONFIG")
	if path == "" {
		return nil, nil, fingerprint, nil
	}
	raw, err := githubPrivateFile(path)
	if err != nil {
		return nil, nil, fingerprint, err
	}
	var registry map[string]githubAppConfig
	if json.Unmarshal(raw, &registry) != nil || registry == nil || len(registry) > 64 {
		return nil, nil, fingerprint, errors.New("invalid private GitHub App registry")
	}
	config, found := registry[v.TokenEnv]
	if !found {
		return nil, nil, fingerprint, nil
	}
	if !regexp.MustCompile(`^[A-Za-z0-9_.-]{1,100}$`).MatchString(config.AppID) || config.InstallationID < 1 || !githubRepo.MatchString(v.Repository) {
		return nil, nil, fingerprint, errors.New("GitHub App requires issuer, installation and Connector repository")
	}
	keyBytes, err := githubPrivateFile(config.PrivateKeyFile)
	if err != nil {
		return nil, nil, fingerprint, err
	}
	block, rest := pem.Decode(keyBytes)
	if block == nil || len(bytes.TrimSpace(rest)) != 0 {
		return nil, nil, fingerprint, errors.New("invalid GitHub App RSA private key")
	}
	key, err := x509.ParsePKCS1PrivateKey(block.Bytes)
	if err != nil {
		parsed, parseErr := x509.ParsePKCS8PrivateKey(block.Bytes)
		if parseErr == nil {
			key, _ = parsed.(*rsa.PrivateKey)
		}
	}
	if key == nil || key.N.BitLen() < 2048 || key.Validate() != nil {
		return nil, nil, fingerprint, errors.New("invalid GitHub App RSA private key")
	}
	configBytes, _ := json.Marshal(config)
	fingerprint = sha256.Sum256(append(configBytes, keyBytes...))
	return &config, key, fingerprint, nil
}

func githubCredentialConfigured(v Connector) error {
	app, _, _, err := connectorGitHubApp(v)
	if err != nil {
		return err
	}
	if app == nil && os.Getenv(v.TokenEnv) == "" {
		return fmt.Errorf("credential environment %s is not configured", v.TokenEnv)
	}
	return nil
}

type githubAppToken struct {
	fingerprint [32]byte
	token       string
	deadline    time.Time
	pending     chan struct{}
}

var githubAppTokens = struct {
	sync.Mutex
	entries map[string]*githubAppToken
}{entries: make(map[string]*githubAppToken)}

func githubCredential(ctx context.Context, v Connector) (string, error) {
	if err := ctx.Err(); err != nil {
		return "", err
	}
	app, key, fingerprint, err := connectorGitHubApp(v)
	if err != nil {
		return "", err
	}
	if app == nil {
		if token := os.Getenv(v.TokenEnv); token != "" {
			return token, nil
		}
		return "", fmt.Errorf("credential environment %s is not configured", v.TokenEnv)
	}
	cacheKey := v.TokenEnv + ":" + strings.ToLower(v.Repository)
	for {
		githubAppTokens.Lock()
		entry := githubAppTokens.entries[cacheKey]
		if entry != nil && entry.pending != nil {
			pending := entry.pending
			githubAppTokens.Unlock()
			select {
			case <-ctx.Done():
				return "", ctx.Err()
			case <-pending:
				continue
			}
		}
		if entry != nil && entry.fingerprint == fingerprint && time.Now().Before(entry.deadline.Add(-time.Minute)) {
			token := entry.token
			githubAppTokens.Unlock()
			return token, nil
		}
		entry = &githubAppToken{fingerprint: fingerprint, pending: make(chan struct{})}
		githubAppTokens.entries[cacheKey] = entry
		githubAppTokens.Unlock()
		token, expiry, err := mintGitHubAppToken(ctx, *app, key, v.Repository)
		githubAppTokens.Lock()
		if err == nil {
			entry.token = token
			// Keep the duration monotonic if the host clock subsequently rolls back.
			now := time.Now()
			entry.deadline = now.Add(expiry.Sub(now))
		}
		close(entry.pending)
		entry.pending = nil
		githubAppTokens.Unlock()
		return token, err
	}
}

func mintGitHubAppToken(ctx context.Context, app githubAppConfig, key *rsa.PrivateKey, repository string) (string, time.Time, error) {
	encode := base64.RawURLEncoding.EncodeToString
	now := time.Now()
	claims, _ := json.Marshal(map[string]any{"iss": app.AppID, "iat": now.Add(-time.Minute).Unix(), "exp": now.Add(9 * time.Minute).Unix()})
	signed := encode([]byte(`{"alg":"RS256","typ":"JWT"}`)) + "." + encode(claims)
	hash := sha256.Sum256([]byte(signed))
	signature, err := rsa.SignPKCS1v15(rand.Reader, key, crypto.SHA256, hash[:])
	if err != nil {
		return "", time.Time{}, errors.New("GitHub App signing failed")
	}
	body, _ := json.Marshal(map[string]any{"repositories": []string{strings.SplitN(repository, "/", 2)[1]}})
	req, err := http.NewRequestWithContext(ctx, "POST", fmt.Sprintf("https://api.github.com/app/installations/%d/access_tokens", app.InstallationID), bytes.NewReader(body))
	if err != nil {
		return "", time.Time{}, errors.New("GitHub App token request failed")
	}
	req.Header.Set("Authorization", "Bearer "+signed+"."+encode(signature))
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("X-GitHub-Api-Version", "2026-03-10")
	req.Header.Set("Content-Type", "application/json")
	resp, err := githubClient.Do(req)
	if err != nil {
		return "", time.Time{}, errors.New("GitHub App token request did not return a confirmed response")
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusCreated && resp.StatusCode != http.StatusOK {
		return "", time.Time{}, fmt.Errorf("GitHub App token request returned HTTP %d; check installation and permissions", resp.StatusCode)
	}
	raw, err := io.ReadAll(io.LimitReader(resp.Body, 65537))
	var result struct {
		Token  string    `json:"token"`
		Expiry time.Time `json:"expires_at"`
	}
	if err != nil || len(raw) > 65536 || json.Unmarshal(raw, &result) != nil || result.Token == "" || strings.ContainsAny(result.Token, "\r\n") || !result.Expiry.After(time.Now()) {
		return "", time.Time{}, errors.New("GitHub App token response is invalid or expired")
	}
	return result.Token, result.Expiry, nil
}
