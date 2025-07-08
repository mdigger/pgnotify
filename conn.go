// Package pgnotify implements the core notification listening logic.
package pgnotify

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

// connection represents an active PostgreSQL listening connection
type connection struct {
	*pgx.Conn
	cfg *Config
}

// Listen continuously waits for notifications and sends them to the channel
func (c *connection) Listen(ctx context.Context, ch chan<- *pgconn.Notification) error {
	for {
		n, err := c.WaitNotification(ctx)
		if err != nil {
			return err
		}

		select {
		case ch <- n:
			if c.cfg.Logger != nil {
				c.cfg.Logger.Debug("notification sent", "channel", n.Channel)
			}
		case <-ctx.Done():
			return nil
		}
	}
}

// WaitNotification waits for a single notification with timeout handling
func (c *connection) WaitNotification(ctx context.Context) (*pgconn.Notification, error) {
	for {
		n, err := c.WaitWithTimeout(ctx)
		if err == nil {
			return n, nil
		}

		if ctx.Err() != nil {
			return nil, ctx.Err()
		}

		if !errors.Is(err, context.DeadlineExceeded) {
			return nil, err
		}

		if err := c.Ping(ctx); err != nil {
			return nil, fmt.Errorf("ping failed: %w", err)
		}
	}
}

// WaitWithTimeout waits for notification with configured timeout
func (c *connection) WaitWithTimeout(ctx context.Context) (*pgconn.Notification, error) {
	timeout := c.cfg.WaitTimeout
	if timeout <= 0 {
		timeout = defaultWaitTimeout
	}

	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	return c.WaitForNotification(ctx)
}

// Ping checks connection health with configured timeout
func (c *connection) Ping(ctx context.Context) error {
	timeout := c.cfg.PingTimeout
	if timeout <= 0 {
		timeout = defaultPingTimeout
	}

	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	return c.Conn.Ping(ctx)
}
