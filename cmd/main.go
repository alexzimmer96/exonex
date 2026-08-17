package main

import (
	"context"
	"log/slog"
	"os"

	"github.com/alexzimmer96/exonex/internal/cortex"
	"github.com/jackc/pgx/v5/pgxpool"
)

func main() {
	pool, err := pgxpool.New(context.Background(), "postgres://exonex:exonex@localhost:5432/exonex")
	if err != nil {
		slog.Error("failed to connect to database", slog.String("error", err.Error()))
		os.Exit(1)
	}
	defer pool.Close()
	srv := cortex.NewServer(":8080", pool)
	srv.ListenAndServe()
}
