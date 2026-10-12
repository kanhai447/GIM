package ws

import (
	"errors"
	"net/http"
	"net/url"
	"strings"
)

type OriginPolicy struct{ allowed map[string]struct{} }

func NewOriginPolicy(values []string) (*OriginPolicy, error) {
	allowed := make(map[string]struct{}, len(values))
	for _, value := range values {
		origin, err := normalizeOrigin(value)
		if err != nil {
			return nil, err
		}
		allowed[origin] = struct{}{}
	}
	if len(allowed) == 0 {
		return nil, errors.New("group websocket requires at least one allowed origin")
	}
	return &OriginPolicy{allowed: allowed}, nil
}

func (policy *OriginPolicy) Allows(request *http.Request) bool {
	if policy == nil || request == nil {
		return false
	}
	values := request.Header.Values("Origin")
	if len(values) != 1 {
		return false
	}
	origin, err := normalizeOrigin(values[0])
	if err != nil {
		return false
	}
	_, ok := policy.allowed[origin]
	return ok
}

func normalizeOrigin(value string) (string, error) {
	parsed, err := url.Parse(strings.TrimSpace(value))
	if err != nil || parsed.User != nil || parsed.Host == "" || parsed.RawQuery != "" || parsed.Fragment != "" || parsed.Path != "" && parsed.Path != "/" {
		return "", errors.New("invalid group websocket origin")
	}
	scheme := strings.ToLower(parsed.Scheme)
	if scheme != "http" && scheme != "https" {
		return "", errors.New("invalid group websocket origin")
	}
	return scheme + "://" + strings.ToLower(parsed.Host), nil
}
