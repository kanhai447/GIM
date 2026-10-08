package tests

import (
	"bytes"
	"context"
	"encoding/json"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	userv1 "github.com/kanhai447/GIM/server/api/user/v1"
	"github.com/kanhai447/GIM/server/internal/auth/credential"
	authservice "github.com/kanhai447/GIM/server/internal/auth/service"
	"github.com/kanhai447/GIM/server/internal/auth/token"
	authhttp "github.com/kanhai447/GIM/server/internal/auth/transport/http"
	"github.com/kanhai447/GIM/server/internal/auth/userclient"
	"github.com/kanhai447/GIM/server/internal/gateway"
	"github.com/kanhai447/GIM/server/internal/gateway/authclient"
	"github.com/kanhai447/GIM/server/internal/gateway/proxy"
	platformconfig "github.com/kanhai447/GIM/server/internal/platform/config"
	"github.com/kanhai447/GIM/server/internal/platform/discovery"
	platformetcd "github.com/kanhai447/GIM/server/internal/platform/etcd"
	"github.com/kanhai447/GIM/server/internal/user/domain"
	"github.com/kanhai447/GIM/server/internal/user/repository"
	userservice "github.com/kanhai447/GIM/server/internal/user/service"
	usergrpc "github.com/kanhai447/GIM/server/internal/user/transport/grpc"
	userhttp "github.com/kanhai447/GIM/server/internal/user/transport/http"
	clientv3 "go.etcd.io/etcd/client/v3"
	googlegrpc "google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/test/bufconn"
)

func TestGatewayAuthUserChain(t *testing.T) {
	envFile := os.Getenv("GIM_ENV_FILE")
	if envFile == "" {
		t.Skip("set GIM_ENV_FILE to run Gateway/Auth/User integration tests")
	}
	values, err := platformconfig.LoadFile(envFile)
	if err != nil {
		t.Fatalf("load local configuration: %v", err)
	}
	etcdConfig, err := platformetcd.FromValues(values)
	if err != nil {
		t.Fatalf("build etcd configuration: %v", err)
	}
	startupContext, startupCancel := context.WithTimeout(context.Background(), 5*time.Second)
	etcdClient, err := platformetcd.Open(startupContext, etcdConfig)
	startupCancel()
	if err != nil {
		t.Fatalf("open etcd client: %v", err)
	}
	t.Cleanup(func() { _ = etcdClient.Close() })

	userRepository := newMemoryUserRepository()
	userService := userservice.New(userRepository)
	rpcClient, stopRPC := startUserRPC(t, userService)
	t.Cleanup(stopRPC)

	authSecret := bytes.Repeat([]byte{0x5a}, 32)
	tokenManager, err := token.NewManager(authSecret, time.Hour)
	if err != nil {
		t.Fatalf("create token manager: %v", err)
	}
	revocations := newMemoryRevocations()
	authService := authservice.New(userclient.New(rpcClient), credential.NewPasswords(4), tokenManager, revocations, authservice.DefaultAllowlist())
	authHandler := authhttp.NewHandler(authService)
	authMux := http.NewServeMux()
	authMux.HandleFunc("/api/auth/register", authHandler.Register)
	authMux.HandleFunc("/api/auth/login", authHandler.Login)
	authMux.HandleFunc("/api/auth/authentication", authHandler.Authentication)
	authMux.HandleFunc("/api/auth/logout", authHandler.Logout)
	authServer := httptest.NewServer(authMux)
	t.Cleanup(authServer.Close)

	profileHandler := userhttp.NewProfileHandler(userService)
	var observedRole string
	var observedUserID string
	var observedValidPath string
	var observedMutex sync.Mutex
	userServer := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		observedMutex.Lock()
		observedRole = request.Header.Get("Role")
		observedUserID = request.Header.Get("User-ID")
		observedValidPath = request.Header.Get("ValidPath")
		observedMutex.Unlock()
		profileHandler.ServeHTTP(writer, request)
	}))
	t.Cleanup(userServer.Close)

	registry, err := discovery.NewRegistry(etcdClient.Raw(), 10*time.Second)
	if err != nil {
		t.Fatalf("create registry: %v", err)
	}
	lifecycleContext, lifecycleCancel := context.WithCancel(context.Background())
	authRegistration := registerTestService(t, registry, lifecycleContext, "auth_api", "a-checkpoint5-auth", authServer.URL)
	userRegistration := registerTestService(t, registry, lifecycleContext, "user_api", "a-checkpoint5-user", userServer.URL)
	t.Cleanup(func() {
		cleanupContext, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		_ = userRegistration.Close(cleanupContext)
		_ = authRegistration.Close(cleanupContext)
		lifecycleCancel()
	})

	resolver := discovery.NewResolver(discovery.NewEtcdBackend(etcdClient.Raw()))
	transport := http.DefaultTransport.(*http.Transport).Clone()
	gatewayHandler := gateway.NewHandler(
		authclient.New(resolver, &http.Client{Transport: transport}, 2*time.Second),
		resolver,
		proxy.New(transport, 3*time.Second),
		2*time.Second,
	)
	gatewayServer := httptest.NewServer(gatewayHandler)
	t.Cleanup(gatewayServer.Close)

	registerResponse := gatewayRequest(t, gatewayServer.Client(), http.MethodPost, gatewayServer.URL+"/api/auth/register", []byte(`{"account":"gateway.integration","nickname":"Gateway User","pwd":"integration-pass","rePwd":"integration-pass"}`), "", false)
	defer registerResponse.Body.Close()
	registerEnvelope := decodeGatewayEnvelope(t, registerResponse)
	if registerResponse.StatusCode != http.StatusOK || registerEnvelope.Code != 0 {
		t.Fatalf("register through Gateway failed: status=%d code=%d", registerResponse.StatusCode, registerEnvelope.Code)
	}

	loginResponse := gatewayRequest(t, gatewayServer.Client(), http.MethodPost, gatewayServer.URL+"/api/auth/login", []byte(`{"account":"gateway.integration","password":"integration-pass"}`), "", false)
	defer loginResponse.Body.Close()
	loginEnvelope := decodeGatewayEnvelope(t, loginResponse)
	if loginResponse.StatusCode != http.StatusOK || loginEnvelope.Code != 0 {
		t.Fatalf("login through Gateway failed: status=%d code=%d", loginResponse.StatusCode, loginEnvelope.Code)
	}
	var loginData struct {
		Token string `json:"token"`
		User  struct {
			UserID uint64 `json:"userID"`
			Role   int32  `json:"role"`
		} `json:"user"`
	}
	if err := json.Unmarshal(loginEnvelope.Data, &loginData); err != nil || loginData.Token == "" || loginData.User.UserID == 0 || loginData.User.Role != 2 {
		t.Fatal("login response did not contain a valid member identity")
	}

	profileResponse := gatewayRequest(t, gatewayServer.Client(), http.MethodGet, gatewayServer.URL+"/api/user/user_info", nil, loginData.Token, true)
	defer profileResponse.Body.Close()
	profileEnvelope := decodeGatewayEnvelope(t, profileResponse)
	if profileResponse.StatusCode != http.StatusOK || profileEnvelope.Code != 0 {
		t.Fatalf("profile through Gateway failed: status=%d code=%d", profileResponse.StatusCode, profileEnvelope.Code)
	}
	observedMutex.Lock()
	role, userID, validPath := observedRole, observedUserID, observedValidPath
	observedMutex.Unlock()
	if role != "2" || userID != strconv.FormatUint(loginData.User.UserID, 10) || validPath != "" {
		t.Fatalf("Gateway forwarded identity role=%q userID=%q validPath=%q", role, userID, validPath)
	}

	missingToken := gatewayRequest(t, gatewayServer.Client(), http.MethodGet, gatewayServer.URL+"/api/user/user_info", nil, "", false)
	defer missingToken.Body.Close()
	missingEnvelope := decodeGatewayEnvelope(t, missingToken)
	if missingToken.StatusCode != http.StatusUnauthorized || missingEnvelope.Code != authservice.CodeTokenMissing {
		t.Fatalf("missing-token response status=%d code=%d", missingToken.StatusCode, missingEnvelope.Code)
	}

	malformedToken := gatewayRequest(t, gatewayServer.Client(), http.MethodGet, gatewayServer.URL+"/api/user/user_info", nil, "invalid-credential", false)
	defer malformedToken.Body.Close()
	malformedEnvelope := decodeGatewayEnvelope(t, malformedToken)
	if malformedToken.StatusCode != http.StatusUnauthorized || malformedEnvelope.Code != authservice.CodeTokenInvalid {
		t.Fatalf("malformed-token response status=%d code=%d", malformedToken.StatusCode, malformedEnvelope.Code)
	}

	wrongSignatureManager, err := token.NewManager(bytes.Repeat([]byte{0x6b}, 32), time.Hour)
	if err != nil {
		t.Fatalf("create wrong-signature manager: %v", err)
	}
	wrongSignatureToken, _, err := wrongSignatureManager.Issue(loginData.User.UserID, loginData.User.Role)
	if err != nil {
		t.Fatalf("issue wrong-signature token: %v", err)
	}
	wrongSignatureResponse := gatewayRequest(t, gatewayServer.Client(), http.MethodGet, gatewayServer.URL+"/api/user/user_info", nil, wrongSignatureToken, false)
	defer wrongSignatureResponse.Body.Close()
	wrongSignatureEnvelope := decodeGatewayEnvelope(t, wrongSignatureResponse)
	if wrongSignatureResponse.StatusCode != http.StatusUnauthorized || wrongSignatureEnvelope.Code != authservice.CodeTokenInvalid {
		t.Fatalf("wrong-signature response status=%d code=%d", wrongSignatureResponse.StatusCode, wrongSignatureEnvelope.Code)
	}

	expiredManager, err := token.NewManagerWithClock(authSecret, time.Hour, func() time.Time { return time.Now().Add(-2 * time.Hour) })
	if err != nil {
		t.Fatalf("create expired-token manager: %v", err)
	}
	expiredToken, _, err := expiredManager.Issue(loginData.User.UserID, loginData.User.Role)
	if err != nil {
		t.Fatalf("issue expired token: %v", err)
	}
	expiredResponse := gatewayRequest(t, gatewayServer.Client(), http.MethodGet, gatewayServer.URL+"/api/user/user_info", nil, expiredToken, false)
	defer expiredResponse.Body.Close()
	expiredEnvelope := decodeGatewayEnvelope(t, expiredResponse)
	if expiredResponse.StatusCode != http.StatusUnauthorized || expiredEnvelope.Code != authservice.CodeTokenExpired {
		t.Fatalf("expired-token response status=%d code=%d", expiredResponse.StatusCode, expiredEnvelope.Code)
	}

	missingService := gatewayRequest(t, gatewayServer.Client(), http.MethodGet, gatewayServer.URL+"/api/chat/history", nil, loginData.Token, false)
	defer missingService.Body.Close()
	missingServiceEnvelope := decodeGatewayEnvelope(t, missingService)
	if missingService.StatusCode != http.StatusServiceUnavailable || missingServiceEnvelope.Code != gateway.CodeServiceUnavailable {
		t.Fatalf("missing-service response status=%d code=%d", missingService.StatusCode, missingServiceEnvelope.Code)
	}

	unavailableEndpoint, _ := url.Parse("http://127.0.0.1:1")
	unavailableRegistration, err := registry.Register(lifecycleContext, "chat_api", "acceptance-unavailable", *unavailableEndpoint)
	if err != nil {
		t.Fatalf("register unavailable upstream: %v", err)
	}
	t.Cleanup(func() {
		cleanupContext, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		_ = unavailableRegistration.Close(cleanupContext)
	})
	unavailableResponse := gatewayRequest(t, gatewayServer.Client(), http.MethodGet, gatewayServer.URL+"/api/chat/history", nil, loginData.Token, false)
	defer unavailableResponse.Body.Close()
	unavailableEnvelope := decodeGatewayEnvelope(t, unavailableResponse)
	if unavailableResponse.StatusCode != http.StatusBadGateway || unavailableEnvelope.Code != proxy.CodeUpstreamUnavailable || strings.Contains(unavailableEnvelope.Msg, unavailableEndpoint.Host) {
		t.Fatalf("unavailable-upstream response status=%d code=%d", unavailableResponse.StatusCode, unavailableEnvelope.Code)
	}

	logoutResponse := gatewayRequest(t, gatewayServer.Client(), http.MethodPost, gatewayServer.URL+"/api/auth/logout", nil, loginData.Token, false)
	defer logoutResponse.Body.Close()
	logoutEnvelope := decodeGatewayEnvelope(t, logoutResponse)
	if logoutResponse.StatusCode != http.StatusOK || logoutEnvelope.Code != 0 {
		t.Fatalf("logout through Gateway failed: status=%d code=%d", logoutResponse.StatusCode, logoutEnvelope.Code)
	}

	revokedResponse := gatewayRequest(t, gatewayServer.Client(), http.MethodGet, gatewayServer.URL+"/api/user/user_info", nil, loginData.Token, false)
	defer revokedResponse.Body.Close()
	revokedEnvelope := decodeGatewayEnvelope(t, revokedResponse)
	if revokedResponse.StatusCode != http.StatusUnauthorized || revokedEnvelope.Code != authservice.CodeTokenRevoked {
		t.Fatalf("revoked-token response status=%d code=%d", revokedResponse.StatusCode, revokedEnvelope.Code)
	}
}

func TestDiscoveryLeaseKeepAliveCleanup(t *testing.T) {
	envFile := os.Getenv("GIM_ENV_FILE")
	if envFile == "" {
		t.Skip("set GIM_ENV_FILE to run discovery lifecycle integration tests")
	}
	values, err := platformconfig.LoadFile(envFile)
	if err != nil {
		t.Fatalf("load local configuration: %v", err)
	}
	etcdConfig, err := platformetcd.FromValues(values)
	if err != nil {
		t.Fatalf("build etcd configuration: %v", err)
	}
	startupContext, startupCancel := context.WithTimeout(context.Background(), 5*time.Second)
	etcdClient, err := platformetcd.Open(startupContext, etcdConfig)
	startupCancel()
	if err != nil {
		t.Fatalf("open etcd client: %v", err)
	}
	t.Cleanup(func() { _ = etcdClient.Close() })

	registry, err := discovery.NewRegistry(etcdClient.Raw(), 2*time.Second)
	if err != nil {
		t.Fatalf("create registry: %v", err)
	}
	lifecycleContext, lifecycleCancel := context.WithCancel(context.Background())
	defer lifecycleCancel()
	instanceID := "lease-check-" + strconv.FormatInt(time.Now().UnixNano(), 10)
	endpoint, _ := url.Parse("http://127.0.0.1:65530")
	registration, err := registry.Register(lifecycleContext, "acceptance_api", instanceID, *endpoint)
	if err != nil {
		t.Fatalf("register acceptance service: %v", err)
	}
	closed := false
	t.Cleanup(func() {
		if closed {
			return
		}
		cleanupContext, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		_ = registration.Close(cleanupContext)
	})

	prefix, err := discovery.ServicePrefix("acceptance_api")
	if err != nil {
		t.Fatalf("build service prefix: %v", err)
	}
	key := prefix + instanceID
	queryContext, queryCancel := context.WithTimeout(context.Background(), 3*time.Second)
	response, err := etcdClient.Raw().Get(queryContext, key)
	queryCancel()
	if err != nil || len(response.Kvs) != 1 || response.Kvs[0].Lease == 0 {
		t.Fatalf("registered lease unavailable: entries=%d err=%v", len(response.Kvs), err)
	}
	leaseID := clientv3.LeaseID(response.Kvs[0].Lease)
	ttlContext, ttlCancel := context.WithTimeout(context.Background(), 3*time.Second)
	ttl, err := etcdClient.Raw().TimeToLive(ttlContext, leaseID)
	ttlCancel()
	if err != nil || ttl == nil || ttl.TTL <= 0 {
		t.Fatalf("lease TTL unavailable: ttl=%v err=%v", ttl, err)
	}

	timer := time.NewTimer(3 * time.Second)
	defer timer.Stop()
	select {
	case <-timer.C:
	case <-lifecycleContext.Done():
		t.Fatal("registration lifecycle ended before keepalive check")
	}
	keepaliveContext, keepaliveCancel := context.WithTimeout(context.Background(), 3*time.Second)
	keepaliveResponse, err := etcdClient.Raw().Get(keepaliveContext, key)
	keepaliveCancel()
	if err != nil || len(keepaliveResponse.Kvs) != 1 {
		t.Fatalf("keepalive did not preserve registration: entries=%d err=%v", len(keepaliveResponse.Kvs), err)
	}

	closeContext, closeCancel := context.WithTimeout(context.Background(), 3*time.Second)
	err = registration.Close(closeContext)
	closeCancel()
	if err != nil {
		t.Fatalf("close registration: %v", err)
	}
	closed = true
	cleanupContext, cleanupCancel := context.WithTimeout(context.Background(), 3*time.Second)
	cleanupResponse, err := etcdClient.Raw().Get(cleanupContext, key)
	cleanupCancel()
	if err != nil || len(cleanupResponse.Kvs) != 0 {
		t.Fatalf("registration cleanup failed: entries=%d err=%v", len(cleanupResponse.Kvs), err)
	}
}

type gatewayEnvelope struct {
	Code uint32          `json:"code"`
	Msg  string          `json:"msg"`
	Data json.RawMessage `json:"data"`
}

func gatewayRequest(t *testing.T, client *http.Client, method, target string, body []byte, tokenValue string, forgedIdentity bool) *http.Response {
	t.Helper()
	request, err := http.NewRequest(method, target, bytes.NewReader(body))
	if err != nil {
		t.Fatalf("create request: %v", err)
	}
	if len(body) > 0 {
		request.Header.Set("Content-Type", "application/json")
	}
	if tokenValue != "" {
		request.Header.Set("token", tokenValue)
	}
	if forgedIdentity {
		request.Header.Set("User-ID", "1")
		request.Header.Set("Role", "1")
		request.Header.Set("ValidPath", "/api/auth/login")
	}
	response, err := client.Do(request)
	if err != nil {
		t.Fatalf("Gateway request failed: %v", err)
	}
	return response
}

func decodeGatewayEnvelope(t *testing.T, response *http.Response) gatewayEnvelope {
	t.Helper()
	var envelope gatewayEnvelope
	if err := json.NewDecoder(response.Body).Decode(&envelope); err != nil {
		t.Fatalf("decode Gateway response: %v", err)
	}
	return envelope
}

func registerTestService(t *testing.T, registry *discovery.Registry, ctx context.Context, service, instanceID, rawURL string) *discovery.Registration {
	t.Helper()
	endpoint, err := url.Parse(rawURL)
	if err != nil {
		t.Fatalf("parse test service URL: %v", err)
	}
	registration, err := registry.Register(ctx, service, instanceID, *endpoint)
	if err != nil {
		t.Fatalf("register %s: %v", service, err)
	}
	return registration
}

func startUserRPC(t *testing.T, service *userservice.Service) (userv1.UserServiceClient, func()) {
	t.Helper()
	listener := bufconn.Listen(1024 * 1024)
	server := googlegrpc.NewServer()
	usergrpc.Register(service)(server)
	serveDone := make(chan error, 1)
	go func() { serveDone <- server.Serve(listener) }()
	connection, err := googlegrpc.NewClient(
		"passthrough:///bufnet",
		googlegrpc.WithTransportCredentials(insecure.NewCredentials()),
		googlegrpc.WithContextDialer(func(context.Context, string) (net.Conn, error) { return listener.Dial() }),
	)
	if err != nil {
		t.Fatalf("create User RPC client: %v", err)
	}
	return userv1.NewUserServiceClient(connection), func() {
		_ = connection.Close()
		server.Stop()
		_ = listener.Close()
		<-serveDone
	}
}

type memoryUserRepository struct {
	mutex     sync.Mutex
	nextID    uint64
	byID      map[uint64]domain.User
	byAccount map[string]uint64
}

func newMemoryUserRepository() *memoryUserRepository {
	return &memoryUserRepository{nextID: 1000, byID: make(map[uint64]domain.User), byAccount: make(map[string]uint64)}
}

func (repositoryMemory *memoryUserRepository) Create(ctx context.Context, user *domain.User) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	repositoryMemory.mutex.Lock()
	defer repositoryMemory.mutex.Unlock()
	if _, exists := repositoryMemory.byAccount[user.Account]; exists {
		return repository.ErrDuplicateAccount
	}
	repositoryMemory.nextID++
	user.ID = repositoryMemory.nextID
	repositoryMemory.byID[user.ID] = *user
	repositoryMemory.byAccount[user.Account] = user.ID
	return nil
}

func (repositoryMemory *memoryUserRepository) GetByID(ctx context.Context, userID uint64) (domain.User, error) {
	if err := ctx.Err(); err != nil {
		return domain.User{}, err
	}
	repositoryMemory.mutex.Lock()
	defer repositoryMemory.mutex.Unlock()
	user, exists := repositoryMemory.byID[userID]
	if !exists {
		return domain.User{}, repository.ErrNotFound
	}
	return user, nil
}

func (repositoryMemory *memoryUserRepository) GetByAccount(ctx context.Context, account string) (domain.User, error) {
	if err := ctx.Err(); err != nil {
		return domain.User{}, err
	}
	repositoryMemory.mutex.Lock()
	defer repositoryMemory.mutex.Unlock()
	userID, exists := repositoryMemory.byAccount[account]
	if !exists {
		return domain.User{}, repository.ErrNotFound
	}
	return repositoryMemory.byID[userID], nil
}

type memoryRevocations struct {
	mutex sync.Mutex
	items map[string]time.Time
}

func newMemoryRevocations() *memoryRevocations {
	return &memoryRevocations{items: make(map[string]time.Time)}
}

func (store *memoryRevocations) Revoke(ctx context.Context, fingerprint string, ttl time.Duration) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	store.mutex.Lock()
	store.items[fingerprint] = time.Now().Add(ttl)
	store.mutex.Unlock()
	return nil
}

func (store *memoryRevocations) IsRevoked(ctx context.Context, fingerprint string) (bool, error) {
	if err := ctx.Err(); err != nil {
		return false, err
	}
	store.mutex.Lock()
	defer store.mutex.Unlock()
	expiresAt, exists := store.items[fingerprint]
	return exists && time.Now().Before(expiresAt), nil
}

var _ repository.Repository = (*memoryUserRepository)(nil)
var _ authservice.Revocations = (*memoryRevocations)(nil)
