package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	_ "github.com/go-sql-driver/mysql"
)

const (
	passwordEnv = "GIM_PERF_PASSWORD"
	dsnEnv      = "GIM_PERF_MYSQL_DSN"
)

var outputWriter io.Writer = os.Stdout

type perfUser struct {
	ID      uint64 `json:"userID"`
	Account string `json:"account"`
}

type perfGroup struct {
	ID        uint64   `json:"id"`
	Size      int      `json:"size"`
	MemberIDs []uint64 `json:"memberIds"`
}

type manifest struct {
	SchemaVersion int         `json:"schemaVersion"`
	Prefix        string      `json:"prefix"`
	BaseURL       string      `json:"baseUrl"`
	Origin        string      `json:"origin"`
	CreatedAt     string      `json:"createdAt"`
	Users         []perfUser  `json:"users"`
	Groups        []perfGroup `json:"groups"`
}

type apiEnvelope struct {
	Code uint32          `json:"code"`
	Msg  string          `json:"msg"`
	Data json.RawMessage `json:"data"`
}

type loginResult struct {
	Token string   `json:"token"`
	User  perfUser `json:"user"`
}

type apiClient struct {
	baseURL string
	http    *http.Client
}

func newAPIClient(baseURL string, timeout time.Duration) (*apiClient, error) {
	parsed, err := url.Parse(strings.TrimRight(baseURL, "/"))
	if err != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") || parsed.Host == "" {
		return nil, errors.New("base URL must be an absolute http/https URL")
	}
	return &apiClient{baseURL: parsed.String(), http: &http.Client{Timeout: timeout}}, nil
}

func (client *apiClient) request(ctx context.Context, method, path, token, contentType string, body io.Reader) (apiEnvelope, int, error) {
	request, err := http.NewRequestWithContext(ctx, method, client.baseURL+path, body)
	if err != nil {
		return apiEnvelope{}, 0, err
	}
	if token != "" {
		request.Header.Set("token", token)
	}
	if contentType != "" {
		request.Header.Set("Content-Type", contentType)
	}
	response, err := client.http.Do(request)
	if err != nil {
		return apiEnvelope{}, 0, err
	}
	defer response.Body.Close()
	payload, err := io.ReadAll(io.LimitReader(response.Body, 2<<20))
	if err != nil {
		return apiEnvelope{}, response.StatusCode, err
	}
	var envelope apiEnvelope
	if err := json.Unmarshal(payload, &envelope); err != nil {
		return apiEnvelope{}, response.StatusCode, fmt.Errorf("invalid API envelope: %w", err)
	}
	return envelope, response.StatusCode, nil
}

func (client *apiClient) json(ctx context.Context, method, path, token string, input any) (apiEnvelope, int, error) {
	var body io.Reader
	if input != nil {
		payload, err := json.Marshal(input)
		if err != nil {
			return apiEnvelope{}, 0, err
		}
		body = strings.NewReader(string(payload))
	}
	return client.request(ctx, method, path, token, "application/json", body)
}

func (client *apiClient) login(ctx context.Context, account, password string) (loginResult, error) {
	envelope, status, err := client.json(ctx, http.MethodPost, "/api/auth/login", "", map[string]string{"account": account, "password": password})
	if err != nil {
		return loginResult{}, err
	}
	if status != http.StatusOK || envelope.Code != 0 {
		return loginResult{}, fmt.Errorf("login rejected: http=%d code=%d", status, envelope.Code)
	}
	var result loginResult
	if err := json.Unmarshal(envelope.Data, &result); err != nil || result.Token == "" || result.User.ID == 0 {
		return loginResult{}, errors.New("login returned invalid data")
	}
	return result, nil
}

func requiredSecret(name string) (string, error) {
	value := os.Getenv(name)
	if value == "" {
		return "", fmt.Errorf("required environment variable %s is not set", name)
	}
	return value, nil
}

func openPerfDB() (*sql.DB, error) {
	dsn, err := requiredSecret(dsnEnv)
	if err != nil {
		return nil, err
	}
	db, err := sql.Open("mysql", dsn)
	if err != nil {
		return nil, errors.New("open performance database failed")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := db.PingContext(ctx); err != nil {
		_ = db.Close()
		return nil, errors.New("performance database health check failed")
	}
	return db, nil
}

func readManifest(path string) (manifest, error) {
	payload, err := os.ReadFile(path)
	if err != nil {
		return manifest{}, err
	}
	var value manifest
	if err := json.Unmarshal(payload, &value); err != nil {
		return manifest{}, err
	}
	if value.SchemaVersion != 1 || value.Prefix == "" || len(value.Users) < 2 {
		return manifest{}, errors.New("invalid performance manifest")
	}
	if value.Origin == "" {
		value.Origin = "http://127.0.0.1:5173"
	}
	return value, nil
}

func writeManifest(path string, value manifest) error {
	payload, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		return err
	}
	return os.WriteFile(path, append(payload, '\n'), 0o600)
}

func timedOut(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return true
	}
	type timeout interface{ Timeout() bool }
	var value timeout
	return errors.As(err, &value) && value.Timeout()
}
