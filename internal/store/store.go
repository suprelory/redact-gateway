package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"path/filepath"
	"time"

	_ "modernc.org/sqlite"
)

type Event struct {
	ID             int64          `json:"id"`
	RequestID      string         `json:"request_id"`
	Timestamp      string         `json:"timestamp"`
	Method         string         `json:"method"`
	Protocol       string         `json:"protocol"`
	UpstreamScheme string         `json:"upstream_scheme"`
	UpstreamHost   string         `json:"upstream_host"`
	UpstreamPort   string         `json:"upstream_port,omitempty"`
	UpstreamPath   string         `json:"upstream_path"`
	Flags          string         `json:"flags"`
	Streaming      bool           `json:"streaming"`
	Status         int            `json:"status"`
	DurationMS     int64          `json:"duration_ms"`
	RequestBytes   int64          `json:"request_bytes"`
	ResponseBytes  int64          `json:"response_bytes"`
	RedactionCount int            `json:"redaction_count"`
	RestoreCount   int            `json:"restore_count"`
	RuleHits       map[string]int `json:"rule_hits"`
	ErrorClass     string         `json:"error_class,omitempty"`
}

type Overview struct {
	Requests   int64   `json:"requests"`
	Redactions int64   `json:"redactions"`
	Restores   int64   `json:"restores"`
	Errors     int64   `json:"errors"`
	AverageMS  float64 `json:"average_ms"`
}

type UpstreamStat struct {
	Host      string  `json:"host"`
	Requests  int64   `json:"requests"`
	Errors    int64   `json:"errors"`
	AverageMS float64 `json:"average_ms"`
}

type Stats struct {
	Overview     Overview       `json:"overview"`
	TopUpstreams []UpstreamStat `json:"top_upstreams"`
}

type Store struct {
	db *sql.DB
}

const allowedHostsSettingKey = "allowed_hosts"

func Open(dataDir string, retentionDays int) (*Store, error) {
	path := filepath.Join(dataDir, "gateway.sqlite3")
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, fmt.Errorf("open sqlite: %w", err)
	}
	db.SetMaxOpenConns(1)
	for _, statement := range []string{
		"PRAGMA journal_mode=WAL",
		"PRAGMA synchronous=NORMAL",
		"PRAGMA busy_timeout=5000",
		"PRAGMA foreign_keys=ON",
	} {
		if _, err := db.Exec(statement); err != nil {
			db.Close()
			return nil, fmt.Errorf("configure sqlite: %w", err)
		}
	}
	store := &Store{db: db}
	if err := store.migrate(); err != nil {
		db.Close()
		return nil, err
	}
	if retentionDays > 0 {
		cutoff := time.Now().AddDate(0, 0, -retentionDays).UnixMilli()
		if _, err := db.Exec("DELETE FROM events WHERE ts_ms < ?", cutoff); err != nil {
			db.Close()
			return nil, fmt.Errorf("apply event retention: %w", err)
		}
	}
	return store, nil
}

func (s *Store) Close() error {
	return s.db.Close()
}

func (s *Store) migrate() error {
	const schema = `
CREATE TABLE IF NOT EXISTS events (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    request_id TEXT NOT NULL,
    ts_ms INTEGER NOT NULL,
    method TEXT NOT NULL,
    protocol TEXT NOT NULL,
    upstream_scheme TEXT NOT NULL,
    upstream_host TEXT NOT NULL,
    upstream_port TEXT NOT NULL,
    upstream_path TEXT NOT NULL,
    flags TEXT NOT NULL,
    streaming INTEGER NOT NULL,
    status INTEGER NOT NULL,
    duration_ms INTEGER NOT NULL,
    request_bytes INTEGER NOT NULL,
    response_bytes INTEGER NOT NULL,
    redaction_count INTEGER NOT NULL,
    restore_count INTEGER NOT NULL,
    rule_hits_json TEXT NOT NULL,
    error_class TEXT NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_events_ts ON events(ts_ms DESC);
CREATE INDEX IF NOT EXISTS idx_events_upstream ON events(upstream_host, ts_ms DESC);
CREATE TABLE IF NOT EXISTS settings (
    key TEXT PRIMARY KEY,
    value_json TEXT NOT NULL,
    updated_at INTEGER NOT NULL
);
`
	if _, err := s.db.Exec(schema); err != nil {
		return fmt.Errorf("migrate sqlite: %w", err)
	}
	return nil
}

func (s *Store) LoadAllowedHosts(ctx context.Context) ([]string, bool, error) {
	var encoded string
	err := s.db.QueryRowContext(ctx, `SELECT value_json FROM settings WHERE key = ?`, allowedHostsSettingKey).Scan(&encoded)
	if errors.Is(err, sql.ErrNoRows) {
		return []string{}, false, nil
	}
	if err != nil {
		return nil, false, fmt.Errorf("query allowed hosts setting: %w", err)
	}
	var hosts []string
	if err := json.Unmarshal([]byte(encoded), &hosts); err != nil {
		return nil, false, fmt.Errorf("decode allowed hosts setting: %w", err)
	}
	if hosts == nil {
		hosts = []string{}
	}
	return hosts, true, nil
}

func (s *Store) SaveAllowedHosts(ctx context.Context, hosts []string) error {
	encoded, err := json.Marshal(hosts)
	if err != nil {
		return fmt.Errorf("encode allowed hosts setting: %w", err)
	}
	_, err = s.db.ExecContext(ctx, `
INSERT INTO settings (key, value_json, updated_at) VALUES (?, ?, ?)
ON CONFLICT(key) DO UPDATE SET value_json = excluded.value_json, updated_at = excluded.updated_at
`, allowedHostsSettingKey, string(encoded), time.Now().UnixMilli())
	if err != nil {
		return fmt.Errorf("save allowed hosts setting: %w", err)
	}
	return nil
}

func (s *Store) InsertEvent(ctx context.Context, event Event) error {
	hits, err := json.Marshal(event.RuleHits)
	if err != nil {
		return fmt.Errorf("encode rule hits: %w", err)
	}
	timestamp := time.Now().UTC()
	if event.Timestamp != "" {
		if parsed, parseErr := time.Parse(time.RFC3339Nano, event.Timestamp); parseErr == nil {
			timestamp = parsed
		}
	}
	_, err = s.db.ExecContext(ctx, `
INSERT INTO events (
    request_id, ts_ms, method, protocol, upstream_scheme, upstream_host,
    upstream_port, upstream_path, flags, streaming, status, duration_ms,
    request_bytes, response_bytes, redaction_count, restore_count,
    rule_hits_json, error_class
) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		event.RequestID, timestamp.UnixMilli(), event.Method, event.Protocol,
		event.UpstreamScheme, event.UpstreamHost, event.UpstreamPort, event.UpstreamPath,
		event.Flags, boolInt(event.Streaming), event.Status, event.DurationMS,
		event.RequestBytes, event.ResponseBytes, event.RedactionCount, event.RestoreCount,
		string(hits), event.ErrorClass,
	)
	if err != nil {
		return fmt.Errorf("insert event: %w", err)
	}
	return nil
}

func (s *Store) Events(ctx context.Context, limit int, upstreamHost string) ([]Event, error) {
	if limit <= 0 || limit > 500 {
		limit = 100
	}
	query := `
SELECT id, request_id, ts_ms, method, protocol, upstream_scheme, upstream_host,
       upstream_port, upstream_path, flags, streaming, status, duration_ms,
       request_bytes, response_bytes, redaction_count, restore_count,
       rule_hits_json, error_class
FROM events`
	args := make([]any, 0, 2)
	if upstreamHost != "" {
		query += " WHERE upstream_host = ?"
		args = append(args, upstreamHost)
	}
	query += " ORDER BY ts_ms DESC LIMIT ?"
	args = append(args, limit)

	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("query events: %w", err)
	}
	defer rows.Close()

	events := make([]Event, 0, limit)
	for rows.Next() {
		var event Event
		var timestamp int64
		var streaming int
		var hitsJSON string
		if err := rows.Scan(
			&event.ID, &event.RequestID, &timestamp, &event.Method, &event.Protocol,
			&event.UpstreamScheme, &event.UpstreamHost, &event.UpstreamPort, &event.UpstreamPath,
			&event.Flags, &streaming, &event.Status, &event.DurationMS, &event.RequestBytes,
			&event.ResponseBytes, &event.RedactionCount, &event.RestoreCount, &hitsJSON,
			&event.ErrorClass,
		); err != nil {
			return nil, fmt.Errorf("scan event: %w", err)
		}
		event.Timestamp = time.UnixMilli(timestamp).UTC().Format(time.RFC3339Nano)
		event.Streaming = streaming != 0
		if err := json.Unmarshal([]byte(hitsJSON), &event.RuleHits); err != nil {
			event.RuleHits = map[string]int{}
		}
		events = append(events, event)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate events: %w", err)
	}
	return events, nil
}

func (s *Store) Stats(ctx context.Context, since time.Time) (Stats, error) {
	var result Stats
	cutoff := since.UnixMilli()
	if err := s.db.QueryRowContext(ctx, `
SELECT COUNT(*),
       COALESCE(SUM(redaction_count), 0),
       COALESCE(SUM(restore_count), 0),
       COALESCE(SUM(CASE WHEN status >= 400 OR error_class <> '' THEN 1 ELSE 0 END), 0),
       COALESCE(AVG(duration_ms), 0)
FROM events WHERE ts_ms >= ?`, cutoff).Scan(
		&result.Overview.Requests, &result.Overview.Redactions, &result.Overview.Restores,
		&result.Overview.Errors, &result.Overview.AverageMS,
	); err != nil {
		return Stats{}, fmt.Errorf("query overview stats: %w", err)
	}

	rows, err := s.db.QueryContext(ctx, `
SELECT upstream_host,
       COUNT(*) AS requests,
       SUM(CASE WHEN status >= 400 OR error_class <> '' THEN 1 ELSE 0 END) AS errors,
       AVG(duration_ms) AS average_ms
FROM events
WHERE ts_ms >= ?
GROUP BY upstream_host
ORDER BY requests DESC, upstream_host ASC
LIMIT 10`, cutoff)
	if err != nil {
		return Stats{}, fmt.Errorf("query upstream stats: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var stat UpstreamStat
		if err := rows.Scan(&stat.Host, &stat.Requests, &stat.Errors, &stat.AverageMS); err != nil {
			return Stats{}, fmt.Errorf("scan upstream stats: %w", err)
		}
		result.TopUpstreams = append(result.TopUpstreams, stat)
	}
	if result.TopUpstreams == nil {
		result.TopUpstreams = []UpstreamStat{}
	}
	return result, rows.Err()
}

func boolInt(value bool) int {
	if value {
		return 1
	}
	return 0
}
