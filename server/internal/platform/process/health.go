package process

import (
	"net/http"

	"github.com/zeromicro/go-zero/rest"
)

// HealthzHandler reports only process readiness. Service commands register it
// after their startup dependency checks have succeeded, so it exposes no
// configuration or dependency details.
func HealthzHandler() http.Handler {
	return http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		writer.Header().Set("Content-Type", "application/json")
		writer.WriteHeader(http.StatusOK)
		_, _ = writer.Write([]byte(`{"status":"ok"}`))
	})
}

func RegisterHealthz(server *rest.Server) {
	server.AddRoute(rest.Route{
		Method:  http.MethodGet,
		Path:    "/healthz",
		Handler: HealthzHandler().ServeHTTP,
	})
}
