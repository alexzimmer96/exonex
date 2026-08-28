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
	"connectrpc.com/validate"
	"github.com/alexzimmer96/exonex/internal/cortex/auth"
	"github.com/alexzimmer96/exonex/internal/cortex/domain/document"
	"github.com/alexzimmer96/exonex/internal/cortex/handler"
	"github.com/alexzimmer96/exonex/internal/cortex/repository"
	"github.com/alexzimmer96/exonex/pkg"
	"github.com/alexzimmer96/exonex/pkg/api/exonex/cortex/v1alpha1/cortexv1alpha1connect"
	"github.com/alexzimmer96/exonex/pkg/grpc"
	"github.com/alexzimmer96/exonex/pkg/sql"
	"github.com/alexzimmer96/exonex/pkg/storage"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Server struct {
	addr string
	mux  *http.ServeMux
	pool *pgxpool.Pool
}

type repositories struct {
	documentRepo *repository.DocumentRepository
}

type services struct {
	documentSvc *document.Service
}

func NewServer(config Config) *Server {
	pool, err := config.GetDatabasePool()
	if err != nil {
		slog.Error("failed to connect to database", slog.String("error", err.Error()))
		os.Exit(1)
	}

	volumeManager, err := config.GetVolumeManager()
	if err != nil {
		slog.Error("failed to initialize volume manager", slog.String("error", err.Error()))
		os.Exit(1)
	}

	filterBuilder, err := sql.NewFilterBuilder()
	if err != nil {
		slog.Error("failed to initiate CEL to SQL filter builder", slog.String("error", err.Error()))
		os.Exit(1)
	}

	repos := initRepositories(pool, filterBuilder)
	svc := initServices(repos, volumeManager)
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
		addr: config.ServerAddr,
		mux:  mux,
		pool: pool,
	}
}

func getInterceptors() connect.Option {
	return connect.WithInterceptors(
		auth.CreateAuthContextInterceptor(),
		auth.CreateMethodPermissionInterceptor(),
		grpc.CreateFieldTypeValidationInterceptor(),
		validate.NewInterceptor(),
	)
}

func initRepositories(pool *pgxpool.Pool, filterBuilder *sql.FilterBuilder) repositories {
	return repositories{
		documentRepo: repository.NewDocumentRepository(pool, filterBuilder),
	}
}

func initServices(repos repositories, volumeManager *storage.VolumeManager) services {
	return services{
		documentSvc: document.NewService(repos.documentRepo, volumeManager),
	}
}

func (srv *Server) ListenAndServe() {
	defer srv.pool.Close()

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	httpServer := pkg.NewSecureHttpServer(srv.addr, srv.mux)

	go func() {
		slog.Info("starting server", slog.String("addr", srv.addr))
		if err := httpServer.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			slog.Error("failed to serve http server", slog.String("error", err.Error()))
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
