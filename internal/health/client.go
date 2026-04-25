package health

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"clickhouse-tui/internal/config"

	_ "github.com/ClickHouse/clickhouse-go/v2"
)

// Metrics holds a snapshot of ClickHouse cluster health metrics.
type Metrics struct {
	// Server info
	Uptime   uint64
	Version  string
	Database string

	// Resource usage
	MemoryUsageMB    float64
	MaxMemoryMB      float64
	MemoryPercent    float64
	CurrentQueryCount int
	CPUPercent       float64

	// Throughput
	QueriesPerSec float64
	InsertedRows  uint64
	ReadRows      uint64

	// Storage
	TotalParts  int
	TotalMerges int
	DiskUsedGB  float64

	// Replication (0 if not replicated)
	ReplicaQueueSize int
	ReplicaLogDelay  int

	// Timestamp
	CollectedAt time.Time
}

// Client connects to a ClickHouse instance and collects health metrics.
type Client struct {
	db   *sql.DB
	conn config.Connection
}

// NewClient creates a new health client for the given connection.
func NewClient(conn config.Connection) (*Client, error) {
	// Ports 8443/443 use HTTPS protocol; 9440 uses native+TLS; others use native plaintext
	scheme := "clickhouse"
	params := "dial_timeout=5s&read_timeout=10s"

	switch conn.Port {
	case "8443", "443":
		scheme = "https"
		params += "&secure=true"
	case "9440":
		params += "&secure=true"
	}

	dsn := fmt.Sprintf("%s://%s:%s@%s:%s/%s?%s",
		scheme, conn.User, conn.Password, conn.Host, conn.Port, conn.Database, params)

	db, err := sql.Open("clickhouse", dsn)
	if err != nil {
		return nil, fmt.Errorf("open connection: %w", err)
	}

	db.SetMaxOpenConns(2)
	db.SetMaxIdleConns(1)
	db.SetConnMaxLifetime(30 * time.Second)

	return &Client{db: db, conn: conn}, nil
}

// Close closes the database connection.
func (c *Client) Close() error {
	if c.db != nil {
		return c.db.Close()
	}
	return nil
}

// Ping checks if the connection is alive.
func (c *Client) Ping(ctx context.Context) error {
	return c.db.PingContext(ctx)
}

// Collect gathers a snapshot of health metrics from the ClickHouse instance.
func (c *Client) Collect(ctx context.Context) (*Metrics, error) {
	m := &Metrics{
		Database:    c.conn.Database,
		CollectedAt: time.Now(),
	}

	// Uptime and version
	row := c.db.QueryRowContext(ctx, "SELECT uptime(), version()")
	if err := row.Scan(&m.Uptime, &m.Version); err != nil {
		return nil, fmt.Errorf("uptime/version: %w", err)
	}

	// Current queries (excluding our own)
	row = c.db.QueryRowContext(ctx,
		"SELECT count() FROM system.processes WHERE query NOT LIKE '%system.processes%'")
	if err := row.Scan(&m.CurrentQueryCount); err != nil {
		m.CurrentQueryCount = 0
	}

	// Memory usage from asynchronous_metric_log
	row = c.db.QueryRowContext(ctx, `
		SELECT value FROM system.asynchronous_metrics
		WHERE metric = 'MemoryTracking'`)
	var memBytes float64
	if err := row.Scan(&memBytes); err == nil {
		m.MemoryUsageMB = memBytes / (1024 * 1024)
	}

	// OS total memory
	row = c.db.QueryRowContext(ctx, `
		SELECT value FROM system.asynchronous_metrics
		WHERE metric = 'OSMemoryTotal'`)
	var totalMemBytes float64
	if err := row.Scan(&totalMemBytes); err == nil {
		m.MaxMemoryMB = totalMemBytes / (1024 * 1024)
		if m.MaxMemoryMB > 0 {
			m.MemoryPercent = (m.MemoryUsageMB / m.MaxMemoryMB) * 100
		}
	}

	// CPU usage from OS metrics
	row = c.db.QueryRowContext(ctx, `
		SELECT value FROM system.asynchronous_metrics
		WHERE metric = 'OSUserTimeCPU'`)
	if err := row.Scan(&m.CPUPercent); err != nil {
		m.CPUPercent = 0
	}

	// Queries per second (last 10 seconds window)
	row = c.db.QueryRowContext(ctx, `
		SELECT count() / 10
		FROM system.query_log
		WHERE event_time >= now() - INTERVAL 10 SECOND
		  AND type = 'QueryFinish'`)
	if err := row.Scan(&m.QueriesPerSec); err != nil {
		m.QueriesPerSec = 0
	}

	// Recent inserted rows (last 10s)
	row = c.db.QueryRowContext(ctx, `
		SELECT sum(written_rows)
		FROM system.query_log
		WHERE event_time >= now() - INTERVAL 10 SECOND
		  AND type = 'QueryFinish'`)
	if err := row.Scan(&m.InsertedRows); err != nil {
		m.InsertedRows = 0
	}

	// Recent read rows (last 10s)
	row = c.db.QueryRowContext(ctx, `
		SELECT sum(read_rows)
		FROM system.query_log
		WHERE event_time >= now() - INTERVAL 10 SECOND
		  AND type = 'QueryFinish'`)
	if err := row.Scan(&m.ReadRows); err != nil {
		m.ReadRows = 0
	}

	// Active parts
	row = c.db.QueryRowContext(ctx,
		"SELECT count() FROM system.parts WHERE active")
	if err := row.Scan(&m.TotalParts); err != nil {
		m.TotalParts = 0
	}

	// Active merges
	row = c.db.QueryRowContext(ctx,
		"SELECT count() FROM system.merges")
	if err := row.Scan(&m.TotalMerges); err != nil {
		m.TotalMerges = 0
	}

	// Disk usage
	row = c.db.QueryRowContext(ctx, `
		SELECT sum(bytes_on_disk) / (1024*1024*1024)
		FROM system.parts WHERE active`)
	if err := row.Scan(&m.DiskUsedGB); err != nil {
		m.DiskUsedGB = 0
	}

	// Replication queue (if replicated tables exist)
	row = c.db.QueryRowContext(ctx, `
		SELECT count() FROM system.replication_queue`)
	if err := row.Scan(&m.ReplicaQueueSize); err != nil {
		m.ReplicaQueueSize = 0
	}

	return m, nil
}
