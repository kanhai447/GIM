package chat

import (
	"errors"
	"net/http"
	"net/url"
	"strings"
)

type OriginPolicy struct{ allowed map[string]struct{} }

func NewOriginPolicy(origins []string) (*OriginPolicy, error) {
	allowed := make(map[string]struct{}, len(origins))
	for _, origin := range origins {
		canonical, err := canonicalOrigin(origin)
		if err != nil {
			return nil, err
		}
		allowed[canonical] = struct{}{}
	}
	if len(allowed) == 0 {
		return nil, errors.New("at least one websocket origin is required")
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
	origin, err := canonicalOrigin(values[0])
	if err != nil {
		return false
	}
	_, ok := policy.allowed[origin]
	return ok
}

func canonicalOrigin(value string) (string, error) {
	parsed, err := url.Parse(strings.TrimSpace(value))
	if err != nil || parsed.User != nil || parsed.Host == "" || parsed.RawQuery != "" || parsed.Fragment != "" || parsed.Path != "" && parsed.Path != "/" {
		return "", errors.New("invalid websocket origin")
	}
	scheme := strings.ToLower(parsed.Scheme)
	if scheme != "http" && scheme != "https" {
		return "", errors.New("invalid websocket origin")
	}
	return scheme + "://" + strings.ToLower(parsed.Host), nil
}
