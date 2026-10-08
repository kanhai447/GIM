// Package discovery provides service registration and lookup through etcd.
package discovery

import (
	"context"
	"errors"
	"net/url"
	"sort"
	"strings"

	clientv3 "go.etcd.io/etcd/client/v3"
)

const servicePrefix = "/gim/services/"

var (
	ErrInvalidService  = errors.New("invalid service name")
	ErrServiceNotFound = errors.New("service is not registered")
	ErrUnavailable     = errors.New("service discovery unavailable")
)

type Entry struct {
	Key   string
	Value string
}

type Backend interface {
	List(context.Context, string) ([]Entry, error)
}

type Endpoint struct {
	InstanceID string
	URL        url.URL
}

type Resolver struct{ backend Backend }

func NewResolver(backend Backend) *Resolver { return &Resolver{backend: backend} }

func (resolver *Resolver) Resolve(ctx context.Context, service string) ([]Endpoint, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	prefix, err := ServicePrefix(service)
	if err != nil {
		return nil, err
	}
	if resolver == nil || resolver.backend == nil {
		return nil, ErrUnavailable
	}
	entries, err := resolver.backend.List(ctx, prefix)
	if err != nil {
		return nil, &operationError{cause: err}
	}
	endpoints := make([]Endpoint, 0, len(entries))
	seen := make(map[string]struct{}, len(entries))
	for _, entry := range entries {
		endpoint, parseErr := parseEndpoint(entry.Value)
		if parseErr != nil {
			continue
		}
		value := endpoint.String()
		if _, exists := seen[value]; exists {
			continue
		}
		seen[value] = struct{}{}
		endpoints = append(endpoints, Endpoint{
			InstanceID: strings.TrimPrefix(entry.Key, prefix),
			URL:        *endpoint,
		})
	}
	if len(endpoints) == 0 {
		return nil, ErrServiceNotFound
	}
	sort.Slice(endpoints, func(left, right int) bool {
		return endpoints[left].InstanceID < endpoints[right].InstanceID
	})
	return endpoints, nil
}

type EtcdBackend struct{ client etcdReader }

type etcdReader interface {
	Get(context.Context, string, ...clientv3.OpOption) (*clientv3.GetResponse, error)
}

func NewEtcdBackend(client etcdReader) *EtcdBackend { return &EtcdBackend{client: client} }

func (backend *EtcdBackend) List(ctx context.Context, prefix string) ([]Entry, error) {
	if backend == nil || backend.client == nil {
		return nil, ErrUnavailable
	}
	response, err := backend.client.Get(ctx, prefix, clientv3.WithPrefix())
	if err != nil {
		return nil, err
	}
	entries := make([]Entry, 0, len(response.Kvs))
	for _, item := range response.Kvs {
		entries = append(entries, Entry{Key: string(item.Key), Value: string(item.Value)})
	}
	return entries, nil
}

type operationError struct{ cause error }

func (err *operationError) Error() string { return ErrUnavailable.Error() }
func (err *operationError) Unwrap() error { return err.cause }

func ServicePrefix(service string) (string, error) {
	if !validName(service) {
		return "", ErrInvalidService
	}
	return servicePrefix + service + "/", nil
}

func serviceKey(service, instanceID string) (string, error) {
	prefix, err := ServicePrefix(service)
	if err != nil || !validName(instanceID) {
		return "", ErrInvalidService
	}
	return prefix + instanceID, nil
}

func validName(value string) bool {
	if value == "" || len(value) > 64 {
		return false
	}
	if value[0] < 'a' || value[0] > 'z' {
		return false
	}
	for index, character := range value {
		if character >= 'a' && character <= 'z' || index > 0 && (character >= '0' && character <= '9' || character == '-' || character == '_') {
			continue
		}
		return false
	}
	return true
}

func parseEndpoint(value string) (*url.URL, error) {
	endpoint, err := url.Parse(strings.TrimSpace(value))
	if err != nil || endpoint.Host == "" || endpoint.User != nil || endpoint.RawQuery != "" || endpoint.Fragment != "" || endpoint.Path != "" && endpoint.Path != "/" || endpoint.Scheme != "http" && endpoint.Scheme != "https" {
		return nil, errors.New("invalid service endpoint")
	}
	endpoint.Path = ""
	return endpoint, nil
}
