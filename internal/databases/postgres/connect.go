package postgres

import (
	"context"
	"strings"

	"github.com/jackc/pgx/v4"
	"github.com/pkg/errors"
	"github.com/wal-g/tracelog"
)

// Connect establishes a connection with a PostgreSQL server with a connection string. See
// pgconn.Connect for details.
func Connect(ctx context.Context, connString string) (*Conn, error) {
	connConfig, err := ParseConfig(connString)
	if err != nil {
		return nil, err
	}
	return connect(ctx, connConfig)
}

// nolint:gocritic
func tryConnectToGpSegment(config pgx.ConnConfig) (*pgx.Conn, error) {
	config.RuntimeParams["gp_role"] = "utility"
	conn, err := pgx.Connect(context.Background(), config)

	if err != nil {
		config.RuntimeParams["gp_session_role"] = "utility"
		conn, err = pgx.Connect(context.Background(), config)
	}
	return conn, err
}