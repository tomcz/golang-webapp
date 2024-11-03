package sqlite

import (
	"context"
	"database/sql"
	"encoding/base64"
	"errors"
	"fmt"
	log "log/slog"
	"time"

	_ "github.com/mattn/go-sqlite3"
	"k8s.io/utils/clock"

	"github.com/tomcz/golang-webapp/internal/webapp"
	"github.com/tomcz/golang-webapp/internal/webapp/sessions"
)

const schemaSQL = `
create table if not exists sessions (
    session_key   text      primary key,
    session_value text      not null,
    created_at    text      not null,
    expire_at     timestamp not null
);
`

const setSessionSQL = `
INSERT INTO sessions (session_key, session_value, created_at, expire_at) VALUES (?, ?, ?, ?)
ON CONFLICT(session_key) DO UPDATE SET
    session_value = excluded.session_value,
    expire_at = excluded.expire_at
`

const (
	getSessionSQL = `SELECT session_value FROM sessions WHERE session_key = ? AND expire_at > ?`
	deleteKeySQL  = `DELETE FROM sessions WHERE session_key = ?`
	expireSQL     = `DELETE FROM sessions WHERE expire_at < ?`
)

type sqliteCodec struct {
	db    *sql.DB
	clock clock.WithTicker
}

func New(ctx context.Context, dbFile string) (webapp.SessionCodec, error) {
	dsn := fmt.Sprintf("file:%s", dbFile)
	db, err := sql.Open("sqlite3", dsn)
	if err != nil {
		return nil, err
	}
	_, err = db.ExecContext(ctx, schemaSQL)
	if err != nil {
		db.Close()
		return nil, err
	}
	codec := &sqliteCodec{
		db:    db,
		clock: clock.RealClock{},
	}
	go codec.regularDatabaseCleanup(ctx)
	return codec, nil
}

func (s *sqliteCodec) Close() error {
	return s.db.Close()
}

func (s *sqliteCodec) Encode(ctx context.Context, key string, session map[string]any, maxAge time.Duration) (string, error) {
	data, err := sessions.Encode(session)
	if err != nil {
		return "", err
	}
	value := base64.StdEncoding.EncodeToString(data)

	if !sessions.ValidKey(key) {
		key = sessions.RandomKey()
	}

	now := s.clock.Now()
	_, err = s.db.ExecContext(ctx, setSessionSQL, key, value, now, now.Add(maxAge))
	if err != nil {
		return "", err
	}
	return key, nil
}

func (s *sqliteCodec) Decode(ctx context.Context, key string) (map[string]any, error) {
	if !sessions.ValidKey(key) {
		return nil, errors.New("invalid session key")
	}

	var value string
	err := s.db.QueryRowContext(ctx, getSessionSQL, key, s.clock.Now()).Scan(&value)
	if err != nil {
		return nil, err
	}

	buf, err := base64.StdEncoding.DecodeString(value)
	if err != nil {
		return nil, err
	}
	return sessions.Decode(buf)
}

func (s *sqliteCodec) Clear(ctx context.Context, key string) {
	if sessions.ValidKey(key) {
		_, _ = s.db.ExecContext(ctx, deleteKeySQL, key)
	}
}

func (s *sqliteCodec) regularDatabaseCleanup(ctx context.Context) {
	ticker := s.clock.NewTicker(10 * time.Minute)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case now := <-ticker.C():
			s.expireSessions(ctx, now)
		}
	}
}

func (s *sqliteCodec) expireSessions(ctx context.Context, now time.Time) {
	_, err := s.db.ExecContext(ctx, expireSQL, now)
	if err != nil {
		log.Warn("expire sessions failed", "component", "sqlite", "error", err)
	}
}
