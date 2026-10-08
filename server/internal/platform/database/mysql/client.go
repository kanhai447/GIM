// Package mysql owns GIM's reusable MySQL/GORM client lifecycle.
package mysql

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net"
	"strconv"
	"time"

	mysqldriver "github.com/go-sql-driver/mysql"
	"github.com/kanhai447/GIM/server/internal/platform/config"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

const (
	defaultMaxOpenConns    = 25
	defaultMaxIdleConns    = 10
	defaultConnMaxLifetime = 30 * time.Minute
)

type Config struct {
	Host            string
	Port            int
	User            string
	Password        string
	Database        string
	MaxOpenConns    int
	MaxIdleConns    int
	ConnMaxLifetime time.Duration
}

type Client struct {
	db    *gorm.DB
	sqlDB *sql.DB
}

type operationError struct {
	operation string
	cause     error
}

func (e *operationError) Error() string { return "mysql " + e.operation + " failed" }
func (e *operationError) Unwrap() error { return e.cause }

func FromValues(values config.Values) (Config, error) {
	host, err := values.Required("MYSQL_HOST")
	if err != nil {
		return Config{}, err
	}
	port, err := values.Int("MYSQL_PORT")
	if err != nil || port < 1 || port > 65535 {
		return Config{}, errors.New("configuration key MYSQL_PORT must be a valid port")
	}
	user, err := values.Required("MYSQL_USER")
	if err != nil {
		return Config{}, err
	}
	password, err := values.Required("MYSQL_PASSWORD")
	if err != nil {
		return Config{}, err
	}
	database, err := values.Required("MYSQL_DATABASE")
	if err != nil {
		return Config{}, err
	}
	maxOpen, err := optionalInt(values, "MYSQL_MAX_OPEN_CONNS", defaultMaxOpenConns)
	if err != nil || maxOpen < 1 {
		return Config{}, errors.New("configuration key MYSQL_MAX_OPEN_CONNS must be positive")
	}
	maxIdle, err := optionalInt(values, "MYSQL_MAX_IDLE_CONNS", defaultMaxIdleConns)
	if err != nil || maxIdle < 0 || maxIdle > maxOpen {
		return Config{}, errors.New("configuration key MYSQL_MAX_IDLE_CONNS is invalid")
	}
	lifetime, err := optionalDuration(values, "MYSQL_CONN_MAX_LIFETIME", defaultConnMaxLifetime)
	if err != nil || lifetime <= 0 {
		return Config{}, errors.New("configuration key MYSQL_CONN_MAX_LIFETIME is invalid")
	}
	return Config{host, port, user, password, database, maxOpen, maxIdle, lifetime}, nil
}

func Open(ctx context.Context, cfg Config) (*Client, error) {
	driverConfig := newDriverConfig(cfg)
	dsn := driverConfig.FormatDSN()

	db, err := gorm.Open(mysql.Open(dsn), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		return nil, &operationError{"initialization", err}
	}
	sqlDB, err := db.DB()
	if err != nil {
		return nil, &operationError{"pool initialization", err}
	}
	configurePool(sqlDB, cfg)

	client := &Client{db: db, sqlDB: sqlDB}
	if err := client.Health(ctx); err != nil {
		_ = client.Close()
		return nil, err
	}
	return client, nil
}

// OpenSQL opens a raw database/sql pool for infrastructure operations such as
// versioned migrations. Callers must close the returned pool. The public error
// text never contains the formatted DSN.
func OpenSQL(ctx context.Context, cfg Config, multiStatements bool) (*sql.DB, error) {
	driverConfig := newDriverConfig(cfg)
	driverConfig.MultiStatements = multiStatements
	db, err := sql.Open("mysql", driverConfig.FormatDSN())
	if err != nil {
		return nil, &operationError{"initialization", err}
	}
	configurePool(db, cfg)
	if err := db.PingContext(ctx); err != nil {
		_ = db.Close()
		return nil, &operationError{"health check", err}
	}
	return db, nil
}

func newDriverConfig(cfg Config) mysqldriver.Config {
	return mysqldriver.Config{
		User:      cfg.User,
		Passwd:    cfg.Password,
		Net:       "tcp",
		Addr:      net.JoinHostPort(cfg.Host, strconv.Itoa(cfg.Port)),
		DBName:    cfg.Database,
		ParseTime: true,
		Loc:       time.Local,
		Params:    map[string]string{"charset": "utf8mb4"},
	}
}

func configurePool(db *sql.DB, cfg Config) {
	db.SetMaxOpenConns(cfg.MaxOpenConns)
	db.SetMaxIdleConns(cfg.MaxIdleConns)
	db.SetConnMaxLifetime(cfg.ConnMaxLifetime)
}

func (c *Client) DB() *gorm.DB { return c.db }

func (c *Client) Health(ctx context.Context) error {
	if c == nil || c.sqlDB == nil {
		return &operationError{"health check", errors.New("client is not initialized")}
	}
	if err := c.sqlDB.PingContext(ctx); err != nil {
		return &operationError{"health check", err}
	}
	return nil
}

func (c *Client) Close() error {
	if c == nil || c.sqlDB == nil {
		return nil
	}
	if err := c.sqlDB.Close(); err != nil {
		return &operationError{"close", err}
	}
	return nil
}

func optionalInt(values config.Values, key string, fallback int) (int, error) {
	value, ok := values.Lookup(key)
	if !ok || value == "" {
		return fallback, nil
	}
	parsed, err := strconv.Atoi(value)
	if err != nil {
		return 0, fmt.Errorf("configuration key %s must be an integer", key)
	}
	return parsed, nil
}

func optionalDuration(values config.Values, key string, fallback time.Duration) (time.Duration, error) {
	value, ok := values.Lookup(key)
	if !ok || value == "" {
		return fallback, nil
	}
	parsed, err := time.ParseDuration(value)
	if err != nil {
		return 0, fmt.Errorf("configuration key %s must be a duration", key)
	}
	return parsed, nil
}
