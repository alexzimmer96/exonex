package cortex

import "github.com/jackc/pgx/v5/pgxpool"

type Server struct {
	pool *pgxpool.Pool
}
