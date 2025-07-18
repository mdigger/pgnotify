# PostgreSQL Notification Listener

[![Go Reference](https://pkg.go.dev/badge/github.com/mdigger/pgnotify.svg)](https://pkg.go.dev/github.com/mdigger/pgnotify)
[![License](https://img.shields.io/badge/license-MIT-blue.svg)](LICENSE)
[![Go Report Card](https://goreportcard.com/badge/github.com/mdigger/pgnotify)](https://goreportcard.com/report/github.com/mdigger/pgnotify)

A robust Go library for listening to PostgreSQL NOTIFY events with automatic reconnection and configurable timeouts.

## Features

- Automatic reconnection on failures
- Configurable timeouts and retry delays
- Context-aware cancellation
- Structured logging support
- Thread-safe operation
- Minimal dependencies

## Installation

```bash
go get github.com/mdigger/pgnotify
```

## Usage

```go
package main

import (
	"context"
	"log/slog"
	"os"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/mdigger/pgnotify"
)

func main() {
	ctx := context.Background()
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))

	// Initialize connection pool
	pool, err := pgxpool.New(ctx, "postgres://user:pass@localhost/db")
	if err != nil {
		logger.Error("failed to create pool", "error", err)
		return
	}
	defer pool.Close()

	// Create notifier configuration
	cfg := pgnotify.New([]string{"channel1", "channel2"}).
		WithRetryDelay(5 * time.Second).
		WithLogger(logger)

	// Create notification channel
	notifyCh := make(chan *pgconn.Notification, 100)

	// Process notifications
	go func(){
	 	for n := range notifyCh {
			logger.Info("received notification", 
				"channel", n.Channel,
				"payload", n.Payload)
		}
	}()

	// Start listening
	if err := cfg.Listen(ctx, pool, notifyCh); err != nil {
		logger.Error("listener failed", "error", err)
		return
	}

	// Stopped listening
}
```

## Configuration Options

| Option | Description | Default |
|--------|-------------|---------|
| Channels | List of channels to listen on | Required |
| RetryDelay | Delay between connection attempts | 10s |
| PingTimeout | Timeout for connection health checks | 5s |
| WaitTimeout | Timeout for waiting on notifications | 20m |
| Logger | Structured logger instance | slog.Default() |

## Best Practices

1. Use buffered channels to prevent notification loss during spikes
2. Set appropriate timeouts for your network conditions
3. Handle context cancellation gracefully in your application
4. Monitor connection errors via the logger

