package cortex

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"connectrpc.com/connect"
	"connectrpc.com/grpcreflect"
	"github.com/alexzimmer96/exonex/internal/auth"
	"github.com/alexzimmer96/exonex/internal/cortex/domain"
	"github.com/alexzimmer96/exonex/internal/cortex/handler"
	"github.com/alexzimmer96/exonex/internal/cortex/repository"
	"github.com/alexzimmer96/exonex/pkg"
	"github.com/alexzimmer96/exonex/pkg/api/exonex/cortex/v1alpha1/cortexv1alpha1connect"
	"github.com/alexzimmer96/exonex/pkg/grpc"
	"github.com/alexzimmer96/exonex/pkg/sql"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Server struct {
	addr string
	mux  *http.ServeMux
}

type repositories struct {
	documentRepo *repository.DocumentRepository
}

type services struct {
	documentSvc *domain.DocumentService
}

func NewServer(addr string, pool *pgxpool.Pool) *Server {
	filterBuilder, err := sql.NewFilterBuilder()
	if err != nil {
		slog.Error("failed to initiate CEL to SQL filter builder", slog.String("error", err.Error()))
		os.Exit(1)
	}

	repos := initRepositories(pool, filterBuilder)
	svc := initServices(repos)
	interceptors := getInterceptors()

	mux := http.NewServeMux()

	reflector := grpcreflect.NewStaticReflector(
		cortexv1alpha1connect.AuthServiceName,
		cortexv1alpha1connect.DocumentServiceName,
		cortexv1alpha1connect.ArtifactServiceName,
	)
	mux.Handle(grpcreflect.NewHandlerV1(reflector))
	mux.Handle(grpcreflect.NewHandlerV1Alpha(reflector))

	mux.Handle(cortexv1alpha1connect.NewAuthServiceHandler(
		handler.NewAuthHandler(),
		interceptors,
	))
	mux.Handle(cortexv1alpha1connect.NewDocumentServiceHandler(
		handler.NewDocumentHandler(svc.documentSvc),
		interceptors,
	))

	return &Server{
		addr: addr,
		mux:  mux,
	}
}

func getInterceptors() connect.Option {
	return connect.WithInterceptors(
		auth.CreateAuthContextInterceptor(),
		auth.CreateMethodPermissionInterceptor(),
		grpc.CreateFieldTypeValidationInterceptor(),
	)
}

func initRepositories(pool *pgxpool.Pool, filterBuilder *sql.FilterBuilder) repositories {
	return repositories{
		documentRepo: repository.NewDocumentRepository(pool, filterBuilder),
	}
}

func initServices(repos repositories) services {
	return services{
		documentSvc: domain.NewDocumentService(repos.documentRepo),
	}
}

func (srv *Server) ListenAndServe() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	httpServer := pkg.NewSecureHttpServer(srv.addr, srv.mux)

	go func() {
		slog.Info("starting server", "addr", httpServer.Addr)
		if err := httpServer.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			slog.Error("failed to serve http server", "error", err)
		}
	}()

	<-ctx.Done()
	slog.Info("initiated http server shutdown")

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if err := httpServer.Shutdown(shutdownCtx); err != nil {
		slog.Info("graceful shutdown timed out")
	}
}
