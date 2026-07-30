package pkg

import (
	"context"
	"database/sql"
	"io/fs"
	"log/slog"

	"github.com/pressly/goose/v3"
)

func Migrate(ctx context.Context, dbUrl string, migrations fs.FS) error {
	db, err := sql.Open("pgx", dbUrl)
	if err != nil {
		slog.ErrorContext(ctx, "error opening database connection", "error", err)
		return err
	}
	defer func(db *sql.DB) {
		_ = db.Close()
	}(db)

	goose.SetBaseFS(migrations)
	if err := goose.SetDialect("postgres"); err != nil {
		slog.ErrorContext(ctx, "error setting goose dialect", "error", err)
		return err
	}

	if err := goose.Up(db, "migrations"); err != nil {
		slog.ErrorContext(ctx, "error running database migrations", "error", err)
		return err
	}

	return nil
}
