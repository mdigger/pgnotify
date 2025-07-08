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

// listen continuously waits for notifications and sends them to the channel
func (c *connection) listen(ctx context.Context, ch chan<- *pgconn.Notification) error {
	for {
		n, err := c.waitNotification(ctx)
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

// waitNotification waits for a single notification with timeout handling
func (c *connection) waitNotification(ctx context.Context) (*pgconn.Notification, error) {
	for {
		n, err := c.waitWithTimeout(ctx)
		if err == nil {
			return n, nil
		}

		if ctx.Err() != nil {
			return nil, ctx.Err()
		}

		if !errors.Is(err, context.DeadlineExceeded) {
			return nil, err
		}

		if err := c.ping(ctx); err != nil {
			return nil, fmt.Errorf("ping failed: %w", err)
		}
	}
}

// waitWithTimeout waits for notification with configured timeout
func (c *connection) waitWithTimeout(ctx context.Context) (*pgconn.Notification, error) {
	timeout := c.cfg.WaitTimeout
	if timeout <= 0 {
		timeout = defaultWaitTimeout
	}

	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	return c.WaitForNotification(ctx)
}

// ping checks connection health with configured timeout
func (c *connection) ping(ctx context.Context) error {
	timeout := c.cfg.PingTimeout
	if timeout <= 0 {
		timeout = defaultPingTimeout
	}

	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	return c.Ping(ctx)
}
