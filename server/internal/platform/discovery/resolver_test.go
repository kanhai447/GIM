package discovery

import (
	"context"
	"errors"
	"strings"
	"testing"
)

type fakeBackend struct {
	entries []Entry
	err     error
	prefix  string
}

func (backend *fakeBackend) List(_ context.Context, prefix string) ([]Entry, error) {
	backend.prefix = prefix
	return backend.entries, backend.err
}

func TestResolverReturnsSortedMultipleEndpoints(t *testing.T) {
	backend := &fakeBackend{entries: []Entry{
		{Key: "/gim/services/user_api/b", Value: "http://127.0.0.1:9002"},
		{Key: "/gim/services/user_api/a", Value: "http://127.0.0.1:9001"},
		{Key: "/gim/services/user_api/invalid", Value: "not-a-url"},
	}}
	endpoints, err := NewResolver(backend).Resolve(context.Background(), "user_api")
	if err != nil {
		t.Fatalf("Resolve() error = %v", err)
	}
	if backend.prefix != "/gim/services/user_api/" || len(endpoints) != 2 || endpoints[0].InstanceID != "a" || endpoints[1].InstanceID != "b" {
		t.Fatalf("Resolve() = %#v, prefix=%q", endpoints, backend.prefix)
	}
}

func TestResolverErrorsAreStableAndSafe(t *testing.T) {
	if _, err := NewResolver(&fakeBackend{}).Resolve(context.Background(), "unknown_api"); !errors.Is(err, ErrServiceNotFound) {
		t.Fatalf("missing Resolve() error = %v", err)
	}
	private := "etcd endpoint credential private-value"
	_, err := NewResolver(&fakeBackend{err: errors.New(private)}).Resolve(context.Background(), "user_api")
	if !errors.Is(err, errors.Unwrap(err)) || strings.Contains(err.Error(), private) || err.Error() != ErrUnavailable.Error() {
		t.Fatalf("unavailable Resolve() error = %v", err)
	}
	if _, err := NewResolver(&fakeBackend{}).Resolve(context.Background(), "../user"); !errors.Is(err, ErrInvalidService) {
		t.Fatalf("invalid Resolve() error = %v", err)
	}
}
