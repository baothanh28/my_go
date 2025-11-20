package pgnotify

import (
	"context"
	"sync"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// PgxProvider implements ConnectionProvider using pgx/v5.
type PgxProvider struct {
	dsn  string
	pool *pgxpool.Pool
	mu   sync.RWMutex
	conn *pgxpool.Conn
}

// NewPgxProvider creates a new PgxProvider with the given DSN.
func NewPgxProvider(ctx context.Context, dsn string) (*PgxProvider, error) {
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		return nil, ErrConnection("pool creation", err)
	}

	// Acquire a dedicated connection for LISTEN/NOTIFY
	conn, err := pool.Acquire(ctx)
	if err != nil {
		pool.Close()
		return nil, ErrConnection("acquire", err)
	}

	return &PgxProvider{
		dsn:  dsn,
		pool: pool,
		conn: conn,
	}, nil
}

// Listen sends a LISTEN command to PostgreSQL.
func (p *PgxProvider) Listen(ctx context.Context, channel string) error {
	p.mu.RLock()
	defer p.mu.RUnlock()

	if p.conn == nil {
		return ErrNotConnected
	}

	_, err := p.conn.Exec(ctx, "LISTEN "+channel)
	return err
}

// Unlisten sends an UNLISTEN command to PostgreSQL.
func (p *PgxProvider) Unlisten(ctx context.Context, channel string) error {
	p.mu.RLock()
	defer p.mu.RUnlock()

	if p.conn == nil {
		return ErrNotConnected
	}

	_, err := p.conn.Exec(ctx, "UNLISTEN "+channel)
	return err
}

// Notify sends a NOTIFY command to PostgreSQL.
func (p *PgxProvider) Notify(ctx context.Context, channel string, payload string) error {
	p.mu.RLock()
	defer p.mu.RUnlock()

	if p.pool == nil {
		return ErrNotConnected
	}

	_, err := p.pool.Exec(ctx, "SELECT pg_notify($1, $2)", channel, payload)
	return err
}

// WaitForNotification waits for a notification from PostgreSQL.
func (p *PgxProvider) WaitForNotification(ctx context.Context) (*Notification, error) {
	p.mu.RLock()
	conn := p.conn
	p.mu.RUnlock()

	if conn == nil {
		return nil, ErrNotConnected
	}

	notification, err := conn.Conn().WaitForNotification(ctx)
	if err != nil {
		return nil, err
	}

	return &Notification{
		Channel:    notification.Channel,
		Payload:    notification.Payload,
		ReceivedAt: time.Now(),
	}, nil
}

// Ping checks if the connection is still alive.
func (p *PgxProvider) Ping(ctx context.Context) error {
	p.mu.RLock()
	defer p.mu.RUnlock()

	if p.pool == nil {
		return ErrNotConnected
	}

	return p.pool.Ping(ctx)
}

// Close closes the connection.
func (p *PgxProvider) Close() error {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.conn != nil {
		p.conn.Release()
		p.conn = nil
	}

	if p.pool != nil {
		p.pool.Close()
		p.pool = nil
	}

	return nil
}

// IsConnected returns true if the connection is active.
func (p *PgxProvider) IsConnected() bool {
	p.mu.RLock()
	defer p.mu.RUnlock()

	return p.conn != nil && p.pool != nil
}

// Reconnect attempts to reconnect to PostgreSQL.
func (p *PgxProvider) Reconnect(ctx context.Context) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	// Close existing connection
	if p.conn != nil {
		p.conn.Release()
		p.conn = nil
	}

	if p.pool != nil {
		p.pool.Close()
		p.pool = nil
	}

	// Create new pool
	pool, err := pgxpool.New(ctx, p.dsn)
	if err != nil {
		return ErrConnection("pool creation", err)
	}

	// Acquire new connection
	conn, err := pool.Acquire(ctx)
	if err != nil {
		pool.Close()
		return ErrConnection("acquire", err)
	}

	p.pool = pool
	p.conn = conn

	return nil
}
