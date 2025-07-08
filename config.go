// Package pgnotify provides a PostgreSQL LISTEN/NOTIFY listener with automatic reconnection.
package pgnotify

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

const (
	defaultRetryDelay  = 10 * time.Second
	defaultPingTimeout = 5 * time.Second
	defaultWaitTimeout = 20 * time.Minute
)

// Config holds configuration for the PostgreSQL notification listener.
type Config struct {
	Channels    []string      // Channels to listen on
	RetryDelay  time.Duration // Delay between connection attempts
	PingTimeout time.Duration // Timeout for connection ping checks
	WaitTimeout time.Duration // Timeout for waiting on notifications
	Logger      *slog.Logger  // Optional structured logger
}

// New creates a new Config with default values.
// channels: List of PostgreSQL channels to listen on.
func New(channels []string) *Config {
	return &Config{
		Channels:    channels,
		RetryDelay:  defaultRetryDelay,
		PingTimeout: defaultPingTimeout,
		WaitTimeout: defaultWaitTimeout,
		Logger:      slog.Default(),
	}
}

// WithRetryDelay sets the delay between reconnection attempts.
// delay: duration to wait before reconnecting after a failure
// Returns the Config for method chaining
func (c *Config) WithRetryDelay(delay time.Duration) *Config {
	c.RetryDelay = delay
	return c
}

// WithPingTimeout sets the timeout for connection health checks.
// timeout: maximum duration to wait for a ping response
// Returns the Config for method chaining
func (c *Config) WithPingTimeout(timeout time.Duration) *Config {
	c.PingTimeout = timeout
	return c
}

// WithWaitTimeout sets the timeout for waiting on notifications.
// timeout: maximum duration to wait for new notifications
// Returns the Config for method chaining
func (c *Config) WithWaitTimeout(timeout time.Duration) *Config {
	c.WaitTimeout = timeout
	return c
}

// WithLogger sets the structured logger implementation.
// logger: structured logger instance (slog.Logger compatible)
// Returns the Config for method chaining
func (c *Config) WithLogger(logger *slog.Logger) *Config {
	c.Logger = logger
	return c
}

// Listen starts listening for notifications and sends them to the provided channel.
// It runs indefinitely until the context is canceled.
// ctx: Context for cancellation
// pool: PostgreSQL connection pool
// notifyCh: Channel to send notifications to
// Returns error if initial setup fails or context is canceled
func (c *Config) Listen(ctx context.Context, pool *pgxpool.Pool, notifyCh chan<- *pgconn.Notification) error {
	if pool == nil || notifyCh == nil {
		return fmt.Errorf("invalid arguments: pool and channel must not be nil")
	}

	for {
		if err := c.listenOnce(ctx, pool, notifyCh); err != nil && c.Logger != nil {
			c.Logger.Error("listen error", "error", err)
		}

		if err := c.sleep(ctx); err != nil {
			return err // Context canceled
		}
	}
}

// listenOnce establishes a single listening session
func (c *Config) listenOnce(ctx context.Context, pool *pgxpool.Pool, notifyCh chan<- *pgconn.Notification) error {
	conn, err := c.connect(ctx, pool)
	if err != nil {
		return err
	}
	defer conn.Close(ctx) //nolint:errcheck

	return conn.listen(ctx, notifyCh)
}

// connect acquires a connection and subscribes to channels
func (c *Config) connect(ctx context.Context, pool *pgxpool.Pool) (*connection, error) {
	poolConn, err := pool.Acquire(ctx)
	if err != nil {
		return nil, fmt.Errorf("acquire connection: %w", err)
	}

	conn := poolConn.Hijack()

	for _, ch := range c.Channels {
		if _, err := conn.Exec(ctx, "LISTEN "+pgx.Identifier{ch}.Sanitize()); err != nil {
			conn.Close(ctx) //nolint:errcheck
			return nil, fmt.Errorf("listen channel %s: %w", ch, err)
		}
	}

	if c.Logger != nil {
		c.Logger.Info("subscribed to channels", "channels", c.Channels)
	}

	return &connection{Conn: conn, cfg: c}, nil
}

// sleep waits for retry delay or context cancellation
func (c *Config) sleep(ctx context.Context) error {
	delay := c.RetryDelay
	if delay <= 0 {
		delay = defaultRetryDelay
	}

	timer := time.NewTimer(delay)
	defer timer.Stop()

	select {
	case <-timer.C:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}
