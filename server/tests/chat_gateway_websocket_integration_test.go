package tests

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strconv"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"github.com/kanhai447/GIM/server/internal/auth/revocation"
	authservice "github.com/kanhai447/GIM/server/internal/auth/service"
	"github.com/kanhai447/GIM/server/internal/auth/token"
	authhttp "github.com/kanhai447/GIM/server/internal/auth/transport/http"
	"github.com/kanhai447/GIM/server/internal/chat"
	"github.com/kanhai447/GIM/server/internal/chat/presence"
	"github.com/kanhai447/GIM/server/internal/gateway"
	"github.com/kanhai447/GIM/server/internal/gateway/authclient"
	"github.com/kanhai447/GIM/server/internal/gateway/proxy"
	platformconfig "github.com/kanhai447/GIM/server/internal/platform/config"
	"github.com/kanhai447/GIM/server/internal/platform/discovery"
	platformetcd "github.com/kanhai447/GIM/server/internal/platform/etcd"
	serviceruntime "github.com/kanhai447/GIM/server/internal/platform/process"
	platformredis "github.com/kanhai447/GIM/server/internal/platform/redis"
	"github.com/kanhai447/GIM/server/internal/platform/rediskeys"
	"github.com/zeromicro/go-zero/core/service"
	"github.com/zeromicro/go-zero/rest"
)

func TestGatewayAuthChatWebSocketTunnel(t *testing.T) {
	envFile := os.Getenv("GIM_ENV_FILE")
	if envFile == "" {
		t.Skip("set GIM_ENV_FILE to run Gateway/Auth/Chat WebSocket integration tests")
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
	redisConfig, err := platformredis.FromValues(values)
	if err != nil {
		t.Fatalf("build Redis configuration: %v", err)
	}
	startupContext, startupCancel = context.WithTimeout(context.Background(), 5*time.Second)
	redisClient, err := platformredis.Open(startupContext, redisConfig)
	startupCancel()
	if err != nil {
		t.Fatalf("open Redis client: %s", redisErrorCategory(err))
	}
	t.Cleanup(func() { _ = redisClient.Close() })

	tokenManager, err := token.NewManager(bytes.Repeat([]byte{0x71}, 32), time.Hour)
	if err != nil {
		t.Fatalf("create token manager: %v", err)
	}
	revocations := newMemoryRevocations()
	authService := authservice.New(nil, nil, tokenManager, revocations, authservice.DefaultAllowlist())
	authHandler := authhttp.NewHandler(authService)
	authMux := http.NewServeMux()
	authMux.HandleFunc("/api/auth/authentication", authHandler.Authentication)
	authMux.HandleFunc("/api/auth/logout", authHandler.Logout)
	authServer := httptest.NewServer(authMux)
	t.Cleanup(authServer.Close)

	suffix := strconv.FormatInt(time.Now().UnixNano(), 10)
	presenceService, err := presence.NewService(
		presence.NewRedisStore(redisClient.Raw()),
		presence.Config{
			InstanceID: "gateway-chat-" + suffix, TTL: 1500 * time.Millisecond,
			RefreshInterval: 300 * time.Millisecond, RetryInterval: 100 * time.Millisecond, OperationTimeout: 200 * time.Millisecond,
		},
		log.New(io.Discard, "", 0),
	)
	if err != nil {
		t.Fatalf("create Presence service: %v", err)
	}
	presenceContext, presenceCancel := context.WithCancel(context.Background())
	go presenceService.Run(presenceContext)
	hub := chat.NewHub(presenceService)
	hubContext, hubCancel := context.WithCancel(context.Background())
	go hub.Run(hubContext)
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		_ = hub.Shutdown(ctx)
		_ = presenceService.Shutdown(ctx)
		hubCancel()
		presenceCancel()
		_ = redisClient.Raw().Del(context.Background(), rediskeys.PresenceUser(72), rediskeys.PresenceUser(73)).Err()
	})
	origins, err := chat.NewOriginPolicy([]string{"http://localhost:5173"})
	if err != nil {
		t.Fatalf("create origin policy: %v", err)
	}
	chatHandler, err := chat.NewHandler(
		hub,
		chat.UnavailableInboundHandler{},
		8,
		origins,
		chat.HeartbeatConfig{ReadLimit: 1 << 20, PongWait: 800 * time.Millisecond, PingPeriod: 200 * time.Millisecond, WriteWait: 100 * time.Millisecond},
		nil,
	)
	if err != nil {
		t.Fatalf("create Chat handler: %v", err)
	}
	chatServerURL := startChatAPI(t, chatHandler)

	registry, err := discovery.NewRegistry(etcdClient.Raw(), 10*time.Second)
	if err != nil {
		t.Fatalf("create registry: %v", err)
	}
	lifecycleContext, lifecycleCancel := context.WithCancel(context.Background())
	authRegistration := registerTestService(t, registry, lifecycleContext, "auth_api", "a-000-day2-auth-"+suffix, authServer.URL)
	chatRegistration := registerTestService(t, registry, lifecycleContext, "chat_api", "a-000-day2-chat-"+suffix, chatServerURL)
	t.Cleanup(func() {
		cleanupContext, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		_ = chatRegistration.Close(cleanupContext)
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

	rawToken, _, err := tokenManager.Issue(72, 2)
	if err != nil {
		t.Fatalf("issue token: %v", err)
	}
	connection := dialGatewayChat(t, gatewayServer.URL, rawToken, http.StatusSwitchingProtocols)
	pingSeen, readerDone := keepGatewayConnectionAlive(connection)
	if count := waitForGatewayChatCount(t, hub, 72, 1); count != 1 {
		t.Fatalf("valid-token connection count = %d", count)
	}
	waitForGatewayPresence(t, presenceService, 72, true)
	secondConnection := dialGatewayChat(t, gatewayServer.URL, rawToken, http.StatusSwitchingProtocols)
	_, secondReaderDone := keepGatewayConnectionAlive(secondConnection)
	if count := waitForGatewayChatCount(t, hub, 72, 2); count != 2 {
		t.Fatalf("two-device connection count = %d", count)
	}
	select {
	case <-pingSeen:
	case <-time.After(2 * time.Second):
		t.Fatal("heartbeat Ping did not survive Gateway tunnel")
	}
	time.Sleep(900 * time.Millisecond)
	if count := waitForGatewayChatCount(t, hub, 72, 2); count != 2 {
		t.Fatalf("Gateway heartbeat connection did not remain alive: %d", count)
	}
	if err := connection.Close(); err != nil {
		t.Fatalf("close valid connection: %v", err)
	}
	select {
	case <-readerDone:
	case <-time.After(time.Second):
		t.Fatal("Gateway client reader did not exit")
	}
	if count := waitForGatewayChatCount(t, hub, 72, 1); count != 1 {
		t.Fatalf("one-device connection count = %d", count)
	}
	waitForGatewayPresence(t, presenceService, 72, true)
	if err := secondConnection.Close(); err != nil {
		t.Fatalf("close second connection: %v", err)
	}
	select {
	case <-secondReaderDone:
	case <-time.After(time.Second):
		t.Fatal("second Gateway client reader did not exit")
	}
	if count := waitForGatewayChatCount(t, hub, 72, 0); count != 0 {
		t.Fatalf("closed final connection count = %d", count)
	}
	waitForGatewayPresence(t, presenceService, 72, false)

	heartbeatToken, _, err := tokenManager.Issue(73, 2)
	if err != nil {
		t.Fatalf("issue heartbeat token: %v", err)
	}
	deadConnection := dialGatewayChat(t, gatewayServer.URL, heartbeatToken, http.StatusSwitchingProtocols)
	healthyConnection := dialGatewayChat(t, gatewayServer.URL, heartbeatToken, http.StatusSwitchingProtocols)
	_, healthyReaderDone := keepGatewayConnectionAlive(healthyConnection)
	if count := waitForGatewayChatCount(t, hub, 73, 2); count != 2 {
		t.Fatalf("heartbeat device count = %d", count)
	}
	waitForGatewayPresence(t, presenceService, 73, true)
	if count := waitForGatewayChatCount(t, hub, 73, 1); count != 1 {
		t.Fatalf("heartbeat sibling timeout count = %d", count)
	}
	waitForGatewayPresence(t, presenceService, 73, true)
	_ = deadConnection.Close()
	_ = healthyConnection.Close()
	select {
	case <-healthyReaderDone:
	case <-time.After(time.Second):
		t.Fatal("healthy heartbeat reader did not exit")
	}
	if count := waitForGatewayChatCount(t, hub, 73, 0); count != 0 {
		t.Fatalf("heartbeat final connection count = %d", count)
	}
	waitForGatewayPresence(t, presenceService, 73, false)

	dialGatewayChat(t, gatewayServer.URL, "", http.StatusUnauthorized)

	logoutRequest, err := http.NewRequest(http.MethodPost, gatewayServer.URL+"/api/auth/logout", nil)
	if err != nil {
		t.Fatalf("create logout request: %v", err)
	}
	logoutRequest.Header.Set("token", rawToken)
	logoutResponse, err := gatewayServer.Client().Do(logoutRequest)
	if err != nil {
		t.Fatalf("logout through Gateway: %v", err)
	}
	_ = logoutResponse.Body.Close()
	if logoutResponse.StatusCode != http.StatusOK {
		t.Fatalf("logout status = %d", logoutResponse.StatusCode)
	}
	if revoked, _ := revocations.IsRevoked(context.Background(), revocation.Fingerprint(rawToken)); !revoked {
		t.Fatal("logout did not revoke token")
	}
	dialGatewayChat(t, gatewayServer.URL, rawToken, http.StatusUnauthorized)
}

func keepGatewayConnectionAlive(connection *websocket.Conn) (<-chan struct{}, <-chan struct{}) {
	pingSeen := make(chan struct{}, 16)
	done := make(chan struct{})
	connection.SetPingHandler(func(payload string) error {
		select {
		case pingSeen <- struct{}{}:
		default:
		}
		return connection.WriteControl(websocket.PongMessage, []byte(payload), time.Now().Add(time.Second))
	})
	go func() {
		defer close(done)
		for {
			if _, _, err := connection.ReadMessage(); err != nil {
				return
			}
		}
	}()
	return pingSeen, done
}

func startChatAPI(t *testing.T, handler *chat.Handler) string {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("reserve Chat API port: %v", err)
	}
	port := listener.Addr().(*net.TCPAddr).Port
	if err := listener.Close(); err != nil {
		t.Fatalf("release Chat API port: %v", err)
	}
	server, err := rest.NewServer(rest.RestConf{
		ServiceConf: service.ServiceConf{Name: "chat-websocket-integration-" + strconv.Itoa(port)},
		Host:        "127.0.0.1",
		Port:        port,
		Timeout:     1000,
	})
	if err != nil {
		t.Fatalf("create Chat API server: %v", err)
	}
	chat.RegisterRoute(server, "/api/chat/ws/chat", handler)
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- serviceruntime.ServeREST(ctx, server, 2*time.Second) }()
	t.Cleanup(func() {
		cancel()
		select {
		case err := <-done:
			if err != nil {
				t.Errorf("stop Chat API: %v", err)
			}
		case <-time.After(3 * time.Second):
			t.Error("Chat API shutdown timed out")
		}
	})
	serverURL := fmt.Sprintf("http://127.0.0.1:%d", port)
	client := &http.Client{Timeout: 200 * time.Millisecond}
	deadline := time.Now().Add(3 * time.Second)
	for {
		response, requestErr := client.Get(serverURL + "/api/chat/ws/chat")
		if requestErr == nil {
			_ = response.Body.Close()
			if response.StatusCode == http.StatusUnauthorized {
				return serverURL
			}
		}
		if time.Now().After(deadline) {
			t.Fatal("Chat API did not become ready")
		}
		time.Sleep(20 * time.Millisecond)
	}
}

func dialGatewayChat(t *testing.T, gatewayURL, rawToken string, expectedStatus int) *websocket.Conn {
	t.Helper()
	endpoint, err := url.Parse(gatewayURL)
	if err != nil {
		t.Fatalf("parse Gateway URL: %v", err)
	}
	endpoint.Scheme = "ws"
	endpoint.Path = "/api/chat/ws/chat"
	if rawToken != "" {
		query := endpoint.Query()
		query.Set("token", rawToken)
		endpoint.RawQuery = query.Encode()
	}
	header := http.Header{}
	header.Set("Origin", "http://localhost:5173")
	connection, response, dialErr := websocket.DefaultDialer.Dial(endpoint.String(), header)
	if expectedStatus == http.StatusSwitchingProtocols {
		if dialErr != nil || connection == nil {
			t.Fatalf("valid WebSocket dial failed: %v", dialErr)
		}
		return connection
	}
	if connection != nil {
		_ = connection.Close()
	}
	if dialErr == nil || response == nil || response.StatusCode != expectedStatus {
		status := 0
		if response != nil {
			status = response.StatusCode
		}
		t.Fatalf("WebSocket rejection status=%d expected=%d err=%v", status, expectedStatus, dialErr)
	}
	_ = response.Body.Close()
	return nil
}

func waitForGatewayChatCount(t *testing.T, hub *chat.Hub, userID uint64, expected int) int {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for {
		ctx, cancel := context.WithTimeout(context.Background(), 250*time.Millisecond)
		count, err := hub.UserConnectionCount(ctx, userID)
		cancel()
		if err == nil && count == expected {
			return count
		}
		if time.Now().After(deadline) {
			return count
		}
		time.Sleep(10 * time.Millisecond)
	}
}

func waitForGatewayPresence(t *testing.T, service *presence.Service, userID uint64, expected bool) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for {
		ctx, cancel := context.WithTimeout(context.Background(), 250*time.Millisecond)
		online, err := service.IsOnline(ctx, userID)
		cancel()
		if err == nil && online == expected {
			return
		}
		if time.Now().After(deadline) {
			t.Fatalf("Presence user %d online=%t err=%v expected=%t", userID, online, err, expected)
		}
		time.Sleep(10 * time.Millisecond)
	}
}
