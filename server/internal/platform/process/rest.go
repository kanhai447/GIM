package process

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/zeromicro/go-zero/core/proc"
	"github.com/zeromicro/go-zero/rest"
)

// ServeREST gives go-zero REST servers an explicit, context-driven shutdown
// path. This is required on Windows, where go-zero's proc signal hooks are a
// no-op, and also makes service lifecycle ownership clear in every command.
func ServeREST(ctx context.Context, server *rest.Server, shutdownTimeout time.Duration) error {
	if ctx == nil || server == nil || shutdownTimeout <= 0 {
		return errors.New("invalid REST server lifecycle configuration")
	}
	ready := make(chan *http.Server, 1)
	serveError := make(chan error, 1)
	go func() {
		defer func() {
			if recovered := recover(); recovered != nil {
				serveError <- fmt.Errorf("REST server failed: %v", recovered)
			}
		}()
		server.StartWithOpts(func(inner *http.Server) { ready <- inner })
		serveError <- nil
	}()

	var inner *http.Server
	select {
	case inner = <-ready:
	case err := <-serveError:
		return err
	case <-ctx.Done():
		timer := time.NewTimer(shutdownTimeout)
		defer timer.Stop()
		select {
		case inner = <-ready:
			return shutdownREST(inner, serveError, shutdownTimeout)
		case err := <-serveError:
			return err
		case <-timer.C:
			return errors.New("REST server startup cancellation timed out")
		}
	}

	select {
	case err := <-serveError:
		return err
	case <-ctx.Done():
		return shutdownREST(inner, serveError, shutdownTimeout)
	}
}

func shutdownREST(inner *http.Server, serveError <-chan error, timeout time.Duration) error {
	shutdownContext, cancel := context.WithTimeout(context.Background(), timeout)
	err := inner.Shutdown(shutdownContext)
	cancel()
	if err != nil {
		return fmt.Errorf("shutdown REST server: %w", err)
	}
	// On Unix this releases go-zero's registered shutdown listener after the
	// explicit Shutdown above. On Windows proc.Shutdown is intentionally a no-op.
	proc.Shutdown()
	timer := time.NewTimer(timeout)
	defer timer.Stop()
	select {
	case err := <-serveError:
		return err
	case <-timer.C:
		return errors.New("REST server shutdown timed out")
	}
}
