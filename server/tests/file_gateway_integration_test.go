package tests

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/kanhai447/GIM/server/internal/auth/credential"
	authservice "github.com/kanhai447/GIM/server/internal/auth/service"
	"github.com/kanhai447/GIM/server/internal/auth/token"
	authhttp "github.com/kanhai447/GIM/server/internal/auth/transport/http"
	"github.com/kanhai447/GIM/server/internal/auth/userclient"
	filemodule "github.com/kanhai447/GIM/server/internal/file"
	filehttp "github.com/kanhai447/GIM/server/internal/file/transport/http"
	"github.com/kanhai447/GIM/server/internal/gateway"
	"github.com/kanhai447/GIM/server/internal/gateway/authclient"
	"github.com/kanhai447/GIM/server/internal/gateway/proxy"
	platformconfig "github.com/kanhai447/GIM/server/internal/platform/config"
	"github.com/kanhai447/GIM/server/internal/platform/discovery"
	platformetcd "github.com/kanhai447/GIM/server/internal/platform/etcd"
	userservice "github.com/kanhai447/GIM/server/internal/user/service"
	"github.com/zeromicro/go-zero/rest/pathvar"
)

func TestFileThroughGatewayAuthAndEtcd(t *testing.T) {
	envFile := os.Getenv("GIM_ENV_FILE")
	if envFile == "" {
		t.Skip("set GIM_ENV_FILE to run File Gateway integration")
	}
	values, err := platformconfig.LoadFile(envFile)
	if err != nil {
		t.Fatal(err)
	}
	etcdConfig, err := platformetcd.FromValues(values)
	if err != nil {
		t.Fatal(err)
	}
	startup, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	etcdClient, err := platformetcd.Open(startup, etcdConfig)
	cancel()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = etcdClient.Close() })
	userService := userservice.New(newMemoryUserRepository())
	rpcClient, stopRPC := startUserRPC(t, userService)
	t.Cleanup(stopRPC)
	tokenManager, err := token.NewManager(bytes.Repeat([]byte{0x4f}, 32), time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	authService := authservice.New(userclient.New(rpcClient), credential.NewPasswords(4), tokenManager, newMemoryRevocations(), authservice.DefaultAllowlist())
	authHandler := authhttp.NewHandler(authService)
	authMux := http.NewServeMux()
	authMux.HandleFunc("/api/auth/register", authHandler.Register)
	authMux.HandleFunc("/api/auth/login", authHandler.Login)
	authMux.HandleFunc("/api/auth/authentication", authHandler.Authentication)
	authMux.HandleFunc("/api/auth/logout", authHandler.Logout)
	authServer := httptest.NewServer(authMux)
	t.Cleanup(authServer.Close)
	db, raw := newChatMessageDatabase(t, values)
	fileModule, err := filemodule.New(db, t.TempDir(), 1<<20)
	if err != nil {
		t.Fatal(err)
	}
	handler := filehttp.NewHandler(fileModule.Service, 1<<20)
	fileMux := http.NewServeMux()
	fileMux.HandleFunc("POST /api/file/file", handler.UploadFile)
	fileMux.HandleFunc("GET /api/file/user-files", handler.List)
	fileMux.HandleFunc("GET /api/file/{id}", func(writer http.ResponseWriter, request *http.Request) {
		handler.Download(writer, pathvar.WithVars(request, map[string]string{"id": request.PathValue("id")}))
	})
	fileServer := httptest.NewServer(fileMux)
	t.Cleanup(fileServer.Close)
	registry, err := discovery.NewRegistry(etcdClient.Raw(), 10*time.Second)
	if err != nil {
		t.Fatal(err)
	}
	lifecycle, lifecycleCancel := context.WithCancel(context.Background())
	authRegistration := registerTestService(t, registry, lifecycle, "auth_api", "file-gateway-auth", authServer.URL)
	fileRegistration := registerTestService(t, registry, lifecycle, "file_api", "file-gateway-api", fileServer.URL)
	t.Cleanup(func() {
		cleanup, done := context.WithTimeout(context.Background(), 3*time.Second)
		defer done()
		_ = fileRegistration.Close(cleanup)
		_ = authRegistration.Close(cleanup)
		lifecycleCancel()
	})
	resolver := discovery.NewResolver(discovery.NewEtcdBackend(etcdClient.Raw()))
	transport := http.DefaultTransport.(*http.Transport).Clone()
	gatewayServer := httptest.NewServer(gateway.NewHandler(authclient.New(resolver, &http.Client{Transport: transport}, 2*time.Second), resolver, proxy.New(transport, 5*time.Second), 2*time.Second))
	t.Cleanup(gatewayServer.Close)
	register := gatewayRequest(t, gatewayServer.Client(), http.MethodPost, gatewayServer.URL+"/api/auth/register", []byte(`{"account":"file.gateway.integration","nickname":"File User","pwd":"integration-pass","rePwd":"integration-pass"}`), "", false)
	register.Body.Close()
	login := gatewayRequest(t, gatewayServer.Client(), http.MethodPost, gatewayServer.URL+"/api/auth/login", []byte(`{"account":"file.gateway.integration","password":"integration-pass"}`), "", false)
	loginEnvelope := decodeGatewayEnvelope(t, login)
	login.Body.Close()
	var loginData struct {
		Token string `json:"token"`
		User  struct {
			UserID uint64 `json:"userID"`
		} `json:"user"`
	}
	if err := json.Unmarshal(loginEnvelope.Data, &loginData); err != nil || loginData.Token == "" {
		t.Fatalf("login data err=%v", err)
	}
	body := &bytes.Buffer{}
	multipartWriter := multipart.NewWriter(body)
	part, _ := multipartWriter.CreateFormFile("file", "gateway.txt")
	_, _ = part.Write([]byte("through gateway"))
	_ = multipartWriter.Close()
	uploadRequest, _ := http.NewRequest(http.MethodPost, gatewayServer.URL+"/api/file/file", body)
	uploadRequest.Header.Set("Content-Type", multipartWriter.FormDataContentType())
	uploadRequest.Header.Set("token", loginData.Token)
	uploadRequest.Header.Set("User-ID", "999999")
	uploadRequest.Header.Set("Role", "1")
	uploadResponse, err := gatewayServer.Client().Do(uploadRequest)
	if err != nil {
		t.Fatal(err)
	}
	uploadEnvelope := decodeGatewayEnvelope(t, uploadResponse)
	uploadResponse.Body.Close()
	var uploadData struct {
		ID  uint64 `json:"id"`
		URL string `json:"url"`
	}
	if err := json.Unmarshal(uploadEnvelope.Data, &uploadData); err != nil || uploadData.ID == 0 {
		t.Fatalf("upload code=%d data=%s err=%v", uploadEnvelope.Code, uploadEnvelope.Data, err)
	}
	var storedUser uint64
	if err := raw.QueryRow(`SELECT user_id FROM user_files WHERE id = ?`, uploadData.ID).Scan(&storedUser); err != nil || storedUser != loginData.User.UserID {
		t.Fatalf("trusted user stored=%d want=%d err=%v", storedUser, loginData.User.UserID, err)
	}
	list := gatewayRequest(t, gatewayServer.Client(), http.MethodGet, gatewayServer.URL+"/api/file/user-files", nil, loginData.Token, true)
	listEnvelope := decodeGatewayEnvelope(t, list)
	list.Body.Close()
	if listEnvelope.Code != 0 {
		t.Fatalf("list code=%d", listEnvelope.Code)
	}
	download := gatewayRequest(t, gatewayServer.Client(), http.MethodGet, gatewayServer.URL+uploadData.URL, nil, loginData.Token, true)
	payload, _ := io.ReadAll(download.Body)
	download.Body.Close()
	if string(payload) != "through gateway" {
		t.Fatalf("download=%q", payload)
	}
	missing := gatewayRequest(t, gatewayServer.Client(), http.MethodGet, gatewayServer.URL+"/api/file/user-files", nil, "", false)
	missingEnvelope := decodeGatewayEnvelope(t, missing)
	missing.Body.Close()
	if missingEnvelope.Code != authservice.CodeTokenMissing {
		t.Fatalf("missing code=%d", missingEnvelope.Code)
	}
	logout := gatewayRequest(t, gatewayServer.Client(), http.MethodPost, gatewayServer.URL+"/api/auth/logout", nil, loginData.Token, false)
	logout.Body.Close()
	revoked := gatewayRequest(t, gatewayServer.Client(), http.MethodGet, gatewayServer.URL+"/api/file/user-files", nil, loginData.Token, false)
	revokedEnvelope := decodeGatewayEnvelope(t, revoked)
	revoked.Body.Close()
	if revokedEnvelope.Code != authservice.CodeTokenRevoked {
		t.Fatalf("revoked code=%d", revokedEnvelope.Code)
	}
}
