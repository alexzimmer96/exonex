package cortex

import (
	"context"
	"crypto/tls"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"connectrpc.com/connect"
	"github.com/alexzimmer96/exonex/internal/auth"
	"github.com/alexzimmer96/exonex/internal/cortex/domain/document"
	"github.com/alexzimmer96/exonex/internal/cortex/grpc"
	"github.com/alexzimmer96/exonex/pkg/api/exonex/cortex/v1alpha1/cortexv1alpha1connect"
)

type Server struct {
	httpServer http.Server
}

func NewServer(addr string) *Server {
	interceptors := connect.WithInterceptors(
		auth.CreateAuthContextInterceptor(),
		auth.CreateMethodPermissionInterceptor(),
	)

	mux := http.NewServeMux()

	mux.Handle(cortexv1alpha1connect.NewAuthServiceHandler(
		grpc.NewAuthHandler(),
		interceptors,
	))

	mux.Handle(cortexv1alpha1connect.NewDocumentServiceHandler(
		grpc.NewDocumentHandler(document.NewService()),
		interceptors,
	))

	tlsConfig := &tls.Config{
		MinVersion: tls.VersionTLS12,
		CipherSuites: []uint16{
			tls.TLS_ECDHE_ECDSA_WITH_AES_256_GCM_SHA384,
			tls.TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384,
			tls.TLS_ECDHE_ECDSA_WITH_CHACHA20_POLY1305,
			tls.TLS_ECDHE_RSA_WITH_CHACHA20_POLY1305,
		},
	}

	return &Server{
		httpServer: http.Server{
			Addr:              addr,
			Handler:           mux,
			TLSConfig:         tlsConfig,
			ReadHeaderTimeout: 2 * time.Second,
			ReadTimeout:       5 * time.Second,
			WriteTimeout:      10 * time.Second,
			IdleTimeout:       120 * time.Second,
			MaxHeaderBytes:    1 << 20,
		},
	}
}

func (srv *Server) ListenAndServe() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	go func() {
		slog.Info("starting server", "addr", srv.httpServer.Addr)
		if err := srv.httpServer.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			slog.Error("failed to serve http server", "error", err)
		}
	}()

	<-ctx.Done()
	slog.Info("initiated http server shutdown")

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if err := srv.httpServer.Shutdown(shutdownCtx); err != nil {
		slog.Info("graceful shutdown timed out")
	}
}
