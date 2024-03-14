package postgres

import (
	"context"
	"strings"

	"github.com/jackc/pgx/v4"
	"github.com/pkg/errors"
	"github.com/wal-g/tracelog"
)

func connect(ctx context.Context, config *pgx.ConnConfig) (c *pgx.Conn, err error) {
	// Default values are set in ParseConfig. Enforce initial creation by ParseConfig rather than setting defaults from
	// zero values.
	if !config.createdByParseConfig {
		panic("config must be created by ParseConfig")
	}

	c = &Conn{
		config:   config,
		connInfo: pgtype.NewConnInfo(),
		logLevel: config.LogLevel,
		logger:   config.Logger,
	}

	// Only install pgx notification system if no other callback handler is present.
	if config.Config.OnNotification == nil {
		config.Config.OnNotification = c.bufferNotifications
	} else {
		if c.shouldLog(LogLevelDebug) {
			c.log(ctx, LogLevelDebug, "pgx notification handler disabled by application supplied OnNotification", map[string]interface{}{"host": config.Config.Host})
		}
	}

	if c.shouldLog(LogLevelInfo) {
		c.log(ctx, LogLevelInfo, "Dialing PostgreSQL server", map[string]interface{}{"host": config.Config.Host})
	}
	c.pgConn, err = pgconn.ConnectConfig(ctx, &config.Config)
	if err != nil {
		if c.shouldLog(LogLevelError) {
			c.log(ctx, LogLevelError, "connect failed", map[string]interface{}{"err": err})
		}
		return nil, err
	}

	c.preparedStatements = make(map[string]*pgconn.StatementDescription)
	c.doneChan = make(chan struct{})
	c.closedChan = make(chan error)
	c.wbuf = make([]byte, 0, 1024)

	if c.config.BuildStatementCache != nil {
		c.stmtcache = c.config.BuildStatementCache(c.pgConn)
	}

	// Replication connections can't execute the queries to
	// populate the c.PgTypes and c.pgsqlAfInet
	if _, ok := config.Config.RuntimeParams["replication"]; ok {
		return c, nil
	}

	return c, nil
}

// Connect establishes a connection with a PostgreSQL server with a connection string. See
// pgconn.Connect for details.
func Connect(ctx context.Context, pgx.connString string) (*pgx.Conn, error) {
	connConfig, err := ParseConfig(pgx.connString)
	if err != nil {
		return nil, err
	}
	return connect(ctx, connConfig)
}

// nolint:gocritic
func tryConnectToGpSegment(config pgx.ConnConfig) (*pgx.Conn, error) {
	config.RuntimeParams["gp_role"] = "utility"
	conn, err := pgx.Connect(context.Background(), pgx.connString)

	if err != nil {
		config.RuntimeParams["gp_session_role"] = "utility"
		conn, err = pgx.Connect(context.Background(), pgx.connString)
	}
	return conn, err
}