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
	"github.com/kanhai447/GIM/server/internal/gateway"
	"github.com/kanhai447/GIM/server/internal/gateway/authclient"
	"github.com/kanhai447/GIM/server/internal/gateway/proxy"
	groupws "github.com/kanhai447/GIM/server/internal/group/ws"
	groupwsprotocol "github.com/kanhai447/GIM/server/internal/group/ws/protocol"
	platformconfig "github.com/kanhai447/GIM/server/internal/platform/config"
	"github.com/kanhai447/GIM/server/internal/platform/discovery"
	platformetcd "github.com/kanhai447/GIM/server/internal/platform/etcd"
	serviceruntime "github.com/kanhai447/GIM/server/internal/platform/process"
	platformredis "github.com/kanhai447/GIM/server/internal/platform/redis"
	"github.com/kanhai447/GIM/server/internal/platform/rediskeys"
	redisv9 "github.com/redis/go-redis/v9"
	"github.com/zeromicro/go-zero/core/service"
	"github.com/zeromicro/go-zero/rest"
)

func TestGatewayAuthGroupWebSocketTunnelAndPresenceBoundary(t *testing.T) {
	envFile := os.Getenv("GIM_ENV_FILE")
	if envFile == "" {
		t.Skip("set GIM_ENV_FILE to run Gateway/Auth/Group WebSocket integration tests")
	}
	values, err := platformconfig.LoadFile(envFile)
	if err != nil {
		t.Fatalf("load local configuration: %v", err)
	}
	etcdConfig, err := platformetcd.FromValues(values)
	if err != nil {
		t.Fatal(err)
	}
	startup, cancelStartup := context.WithTimeout(context.Background(), 5*time.Second)
	etcdClient, err := platformetcd.Open(startup, etcdConfig)
	cancelStartup()
	if err != nil {
		t.Fatalf("open etcd client: %v", err)
	}
	t.Cleanup(func() { _ = etcdClient.Close() })
	redisConfig, err := platformredis.FromValues(values)
	if err != nil {
		t.Fatal(err)
	}
	startup, cancelStartup = context.WithTimeout(context.Background(), 5*time.Second)
	redisClient, err := platformredis.Open(startup, redisConfig)
	cancelStartup()
	if err != nil {
		t.Fatalf("open Redis client: %s", redisErrorCategory(err))
	}
	t.Cleanup(func() { _ = redisClient.Close() })

	const sentinelUserID uint64 = 990000001
	const primaryUserID uint64 = 172
	const timeoutUserID uint64 = 173
	const slowUserID uint64 = 174
	const shutdownUserID uint64 = 175
	sentinelKey := rediskeys.PresenceUser(sentinelUserID)
	primaryKey := rediskeys.PresenceUser(primaryUserID)
	timeoutKey := rediskeys.PresenceUser(timeoutUserID)
	slowKey := rediskeys.PresenceUser(slowUserID)
	shutdownKey := rediskeys.PresenceUser(shutdownUserID)
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	_ = redisClient.Raw().Del(ctx, sentinelKey, primaryKey, timeoutKey, slowKey, shutdownKey).Err()
	if err := redisClient.Raw().ZAdd(ctx, sentinelKey, redisv9.Z{Score: 424242, Member: "group-ws-sentinel"}).Err(); err != nil {
		t.Fatalf("create Presence sentinel: %v", err)
	}
	if err := redisClient.Raw().Expire(ctx, sentinelKey, 10*time.Minute).Err(); err != nil {
		t.Fatalf("expire Presence sentinel: %v", err)
	}
	t.Cleanup(func() {
		_ = redisClient.Raw().Del(context.Background(), sentinelKey, primaryKey, timeoutKey, slowKey, shutdownKey).Err()
	})

	tokenManager, err := token.NewManager(bytes.Repeat([]byte{0x72}, 32), time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	revocations := newMemoryRevocations()
	authService := authservice.New(nil, nil, tokenManager, revocations, authservice.DefaultAllowlist())
	authHandler := authhttp.NewHandler(authService)
	authMux := http.NewServeMux()
	authMux.HandleFunc("/api/auth/authentication", authHandler.Authentication)
	authMux.HandleFunc("/api/auth/logout", authHandler.Logout)
	authServer := httptest.NewServer(authMux)
	t.Cleanup(authServer.Close)

	hub := groupws.NewHub()
	hubContext, cancelHub := context.WithCancel(context.Background())
	go hub.Run(hubContext)
	t.Cleanup(func() {
		shutdown, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		_ = hub.Shutdown(shutdown)
		cancelHub()
	})
	origins, err := groupws.NewOriginPolicy([]string{"http://localhost:5173"})
	if err != nil {
		t.Fatal(err)
	}
	groupHandler, err := groupws.NewHandler(
		hub, groupws.NewFoundationInbound(hub), 8, origins,
		groupws.HeartbeatConfig{ReadLimit: 1 << 20, PongWait: 800 * time.Millisecond, PingPeriod: 200 * time.Millisecond, WriteWait: 100 * time.Millisecond},
		groupws.NewStandardLifecycleObserver(log.New(io.Discard, "", 0)),
	)
	if err != nil {
		t.Fatal(err)
	}
	groupServerURL := startGroupWebSocketAPI(t, groupHandler)

	suffix := strconv.FormatInt(time.Now().UnixNano(), 10)
	registry, err := discovery.NewRegistry(etcdClient.Raw(), 10*time.Second)
	if err != nil {
		t.Fatal(err)
	}
	lifecycleContext, lifecycleCancel := context.WithCancel(context.Background())
	authRegistration := registerTestService(t, registry, lifecycleContext, "auth_api", "a-000-day3-auth-"+suffix, authServer.URL)
	groupRegistration := registerTestService(t, registry, lifecycleContext, "group_api", "a-000-day3-group-"+suffix, groupServerURL)
	t.Cleanup(func() {
		cleanup, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		_ = groupRegistration.Close(cleanup)
		_ = authRegistration.Close(cleanup)
		lifecycleCancel()
	})

	resolver := discovery.NewResolver(discovery.NewEtcdBackend(etcdClient.Raw()))
	transport := http.DefaultTransport.(*http.Transport).Clone()
	gatewayServer := httptest.NewServer(gateway.NewHandler(
		authclient.New(resolver, &http.Client{Transport: transport}, 2*time.Second), resolver,
		proxy.New(transport, 3*time.Second), 2*time.Second,
	))
	t.Cleanup(gatewayServer.Close)

	rawToken, _, err := tokenManager.Issue(primaryUserID, 2)
	if err != nil {
		t.Fatal(err)
	}
	first := dialGatewayGroup(t, gatewayServer.URL, rawToken, http.Header{"User-ID": []string{"999"}, "Role": []string{"1"}}, http.StatusSwitchingProtocols)
	if count := waitForGroupCount(t, hub, primaryUserID, 1); count != 1 {
		t.Fatalf("trusted user count = %d", count)
	}
	if count := waitForGroupCount(t, hub, 999, 0); count != 0 {
		t.Fatalf("spoofed user registered: %d", count)
	}
	if err := first.WriteMessage(websocket.TextMessage, []byte(`{"event":"group.send","requestId":"gateway","clientMsgId":"future","data":{"groupID":1}}`)); err != nil {
		t.Fatal(err)
	}
	_ = first.SetReadDeadline(time.Now().Add(time.Second))
	_, payload, err := first.ReadMessage()
	if err != nil {
		t.Fatal(err)
	}
	envelope, err := groupwsprotocol.Decode(payload)
	if err != nil || envelope.Error == nil || envelope.Error.Code != groupwsprotocol.ErrorUnsupported {
		t.Fatalf("unsupported response = %#v, %v", envelope, err)
	}
	pingSeen, firstDone := keepGatewayConnectionAlive(first)
	second := dialGatewayGroup(t, gatewayServer.URL, rawToken, nil, http.StatusSwitchingProtocols)
	_, secondDone := keepGatewayConnectionAlive(second)
	if count := waitForGroupCount(t, hub, primaryUserID, 2); count != 2 {
		t.Fatalf("multi-device count = %d", count)
	}
	select {
	case <-pingSeen:
	case <-time.After(2 * time.Second):
		t.Fatal("Group Ping did not cross Gateway")
	}
	_ = first.Close()
	select {
	case <-firstDone:
	case <-time.After(time.Second):
		t.Fatal("first reader did not stop")
	}
	if count := waitForGroupCount(t, hub, primaryUserID, 1); count != 1 {
		t.Fatalf("remaining sibling count = %d", count)
	}

	timeoutToken, _, err := tokenManager.Issue(timeoutUserID, 2)
	if err != nil {
		t.Fatal(err)
	}
	dead := dialGatewayGroup(t, gatewayServer.URL, timeoutToken, nil, http.StatusSwitchingProtocols)
	healthy := dialGatewayGroup(t, gatewayServer.URL, timeoutToken, nil, http.StatusSwitchingProtocols)
	_, healthyDone := keepGatewayConnectionAlive(healthy)
	if count := waitForGroupCount(t, hub, timeoutUserID, 2); count != 2 {
		t.Fatalf("timeout pair count = %d", count)
	}
	if count := waitForGroupCount(t, hub, timeoutUserID, 1); count != 1 {
		t.Fatalf("timeout sibling count = %d", count)
	}
	_ = dead.Close()
	_ = healthy.Close()
	select {
	case <-healthyDone:
	case <-time.After(time.Second):
		t.Fatal("healthy timeout reader did not stop")
	}
	if count := waitForGroupCount(t, hub, timeoutUserID, 0); count != 0 {
		t.Fatalf("timeout final count = %d", count)
	}

	dialGatewayGroup(t, gatewayServer.URL, "", nil, http.StatusUnauthorized)
	logoutRequest, _ := http.NewRequest(http.MethodPost, gatewayServer.URL+"/api/auth/logout", nil)
	logoutRequest.Header.Set("token", rawToken)
	logoutResponse, err := gatewayServer.Client().Do(logoutRequest)
	if err != nil {
		t.Fatal(err)
	}
	_ = logoutResponse.Body.Close()
	if logoutResponse.StatusCode != http.StatusOK {
		t.Fatalf("logout status = %d", logoutResponse.StatusCode)
	}
	if revoked, _ := revocations.IsRevoked(context.Background(), revocation.Fingerprint(rawToken)); !revoked {
		t.Fatal("token was not revoked")
	}
	dialGatewayGroup(t, gatewayServer.URL, rawToken, nil, http.StatusUnauthorized)
	_ = second.Close()
	select {
	case <-secondDone:
	case <-time.After(time.Second):
		t.Fatal("second reader did not stop")
	}
	if count := waitForGroupCount(t, hub, primaryUserID, 0); count != 0 {
		t.Fatalf("primary final count = %d", count)
	}

	slowToken, _, err := tokenManager.Issue(slowUserID, 2)
	if err != nil {
		t.Fatal(err)
	}
	slow := dialGatewayGroup(t, gatewayServer.URL, slowToken, nil, http.StatusSwitchingProtocols)
	if count := waitForGroupCount(t, hub, slowUserID, 1); count != 1 {
		t.Fatalf("slow client count = %d", count)
	}
	largePayload := bytes.Repeat([]byte("s"), 256*1024)
	dropped := false
	for attempt := 0; attempt < 200; attempt++ {
		result, sendErr := hub.SendToUser(context.Background(), slowUserID, largePayload)
		if sendErr != nil {
			t.Fatalf("route to slow client: %v", sendErr)
		}
		if result.Dropped == 1 {
			dropped = true
			break
		}
	}
	if !dropped {
		t.Fatal("slow client queue never applied backpressure")
	}
	if count := waitForGroupCount(t, hub, slowUserID, 0); count != 0 {
		t.Fatalf("slow client remained registered: %d", count)
	}
	_ = slow.Close()

	shutdownToken, _, err := tokenManager.Issue(shutdownUserID, 2)
	if err != nil {
		t.Fatal(err)
	}
	shutdownConnection := dialGatewayGroup(t, gatewayServer.URL, shutdownToken, nil, http.StatusSwitchingProtocols)
	_, shutdownDone := keepGatewayConnectionAlive(shutdownConnection)
	if count := waitForGroupCount(t, hub, shutdownUserID, 1); count != 1 {
		t.Fatalf("shutdown client count = %d", count)
	}
	shutdownContext, shutdownCancel := context.WithTimeout(context.Background(), time.Second)
	if err := hub.Shutdown(shutdownContext); err != nil {
		shutdownCancel()
		t.Fatal(err)
	}
	shutdownCancel()
	select {
	case <-shutdownDone:
	case <-time.After(time.Second):
		t.Fatal("shutdown connection did not close")
	}
	_ = shutdownConnection.Close()

	assertGroupPresenceUntouched(t, redisClient.Raw(), sentinelKey, primaryKey, timeoutKey, slowKey, shutdownKey)
}

func assertGroupPresenceUntouched(t *testing.T, client redisv9.Cmdable, sentinelKey string, userKeys ...string) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	score, err := client.ZScore(ctx, sentinelKey, "group-ws-sentinel").Result()
	if err != nil || score != 424242 {
		t.Fatalf("Presence sentinel changed score=%v err=%v", score, err)
	}
	for _, key := range userKeys {
		exists, err := client.Exists(ctx, key).Result()
		if err != nil || exists != 0 {
			t.Fatalf("Group WS mutated Presence key %s exists=%d err=%v", key, exists, err)
		}
	}
}

func startGroupWebSocketAPI(t *testing.T, handler *groupws.Handler) string {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	port := listener.Addr().(*net.TCPAddr).Port
	_ = listener.Close()
	server, err := rest.NewServer(rest.RestConf{
		ServiceConf: service.ServiceConf{Name: "group-websocket-integration-" + strconv.Itoa(port)},
		Host:        "127.0.0.1", Port: port, Timeout: 1000,
	})
	if err != nil {
		t.Fatal(err)
	}
	groupws.RegisterRoute(server, "/api/group/ws/chat", handler)
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- serviceruntime.ServeREST(ctx, server, 2*time.Second) }()
	t.Cleanup(func() {
		cancel()
		select {
		case err := <-done:
			if err != nil {
				t.Errorf("stop Group API: %v", err)
			}
		case <-time.After(3 * time.Second):
			t.Error("Group API shutdown timed out")
		}
	})
	serverURL := fmt.Sprintf("http://127.0.0.1:%d", port)
	deadline := time.Now().Add(3 * time.Second)
	for {
		response, requestErr := (&http.Client{Timeout: 200 * time.Millisecond}).Get(serverURL + "/api/group/ws/chat")
		if requestErr == nil {
			_ = response.Body.Close()
			if response.StatusCode == http.StatusUnauthorized {
				return serverURL
			}
		}
		if time.Now().After(deadline) {
			t.Fatal("Group API did not become ready")
		}
		time.Sleep(20 * time.Millisecond)
	}
}

func dialGatewayGroup(t *testing.T, gatewayURL, rawToken string, extra http.Header, expectedStatus int) *websocket.Conn {
	t.Helper()
	endpoint, err := url.Parse(gatewayURL)
	if err != nil {
		t.Fatal(err)
	}
	endpoint.Scheme = "ws"
	endpoint.Path = "/api/group/ws/chat"
	if rawToken != "" {
		query := endpoint.Query()
		query.Set("token", rawToken)
		endpoint.RawQuery = query.Encode()
	}
	header := http.Header{"Origin": []string{"http://localhost:5173"}}
	for key, values := range extra {
		header[key] = append([]string(nil), values...)
	}
	connection, response, dialErr := websocket.DefaultDialer.Dial(endpoint.String(), header)
	if expectedStatus == http.StatusSwitchingProtocols {
		if dialErr != nil || connection == nil {
			t.Fatalf("valid Group WebSocket dial failed: %v", dialErr)
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
		t.Fatalf("Group WebSocket rejection status=%d expected=%d err=%v", status, expectedStatus, dialErr)
	}
	_ = response.Body.Close()
	return nil
}

func waitForGroupCount(t *testing.T, hub *groupws.Hub, userID uint64, expected int) int {
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
