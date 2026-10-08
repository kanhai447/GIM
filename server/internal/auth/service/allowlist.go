package service

import (
	"net/url"
	"strings"
)

type Allowlist struct{ paths map[string]struct{} }

func NewAllowlist(paths []string) Allowlist {
	items := make(map[string]struct{}, len(paths))
	for _, path := range paths {
		path = strings.TrimSpace(path)
		if path != "" {
			items[path] = struct{}{}
		}
	}
	return Allowlist{paths: items}
}

func DefaultAllowlist() Allowlist {
	return NewAllowlist([]string{
		"/api/auth/login",
		"/api/auth/register",
		"/api/auth/open_login",
		"/api/settings/info",
		"/api/settings/open_login",
	})
}

func (allowlist Allowlist) IsPublic(validPath string) bool {
	validPath = strings.TrimSpace(validPath)
	parsed, err := url.ParseRequestURI(validPath)
	if err != nil || parsed.Host != "" || parsed.Scheme != "" || parsed.Fragment != "" || parsed.RawPath != "" {
		return false
	}
	_, ok := allowlist.paths[parsed.Path]
	return ok
}
