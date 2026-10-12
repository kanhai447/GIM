package http

import (
	"bytes"
	stdhttp "net/http"
	"testing"

	"github.com/zeromicro/go-zero/rest/pathvar"
)

func TestTrustedUserIDRequiresGatewayIdentity(t *testing.T) {
	request, _ := stdhttp.NewRequest(stdhttp.MethodGet, "/api/group/my", nil)
	request.Header.Set("User-ID", "42")
	request.Header.Set("Role", "2")
	if userID, ok := trustedUserID(request); !ok || userID != 42 {
		t.Fatalf("trustedUserID() = %d, %t", userID, ok)
	}
	request.Header.Set("Role", "9")
	if _, ok := trustedUserID(request); ok {
		t.Fatal("untrusted role was accepted")
	}
}

func TestCreatePayloadCannotSupplyOperatorIdentity(t *testing.T) {
	request, _ := stdhttp.NewRequest(stdhttp.MethodPost, "/api/group/group", bytes.NewBufferString(`{"name":"group","ownerId":999}`))
	var input createRequest
	if decodeJSON(request, &input) {
		t.Fatal("ownerId payload field was accepted")
	}
}

func TestPathIDUsesRouterValue(t *testing.T) {
	request, _ := stdhttp.NewRequest(stdhttp.MethodGet, "/api/group/group/55", nil)
	request = pathvar.WithVars(request, map[string]string{"id": "55"})
	if id, ok := pathID(request); !ok || id != 55 {
		t.Fatalf("pathID() = %d, %t", id, ok)
	}
}
