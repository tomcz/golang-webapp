package sqlite

import (
	"context"
	"database/sql"
	"testing"
	"time"

	"gotest.tools/v3/assert"
	clocks "k8s.io/utils/clock/testing"
)

func testDB() (*sql.DB, error) {
	dsn := "file:test.db?mode=memory"
	db, err := sql.Open("sqlite3", dsn)
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(1)
	_, err = db.Exec(schemaSQL)
	if err != nil {
		db.Close()
		return nil, err
	}
	return db, nil
}

func TestCodecRoundTrip(t *testing.T) {
	db, err := testDB()
	assert.NilError(t, err)

	now := time.Now()
	clock := clocks.NewFakeClock(now)

	codec := &sqliteCodec{
		db:    db,
		clock: clock,
	}

	ctx := context.Background()
	data := map[string]any{"wibble": "wobble"}

	key1, err := codec.Encode(ctx, "", data, time.Hour)
	assert.NilError(t, err)

	decoded, err := codec.Decode(ctx, key1)
	assert.NilError(t, err)
	assert.DeepEqual(t, data, decoded)

	key2, err := codec.Encode(ctx, key1, data, time.Hour)
	assert.NilError(t, err)
	assert.Equal(t, key1, key2)

	codec.Clear(ctx, key1)

	_, err = codec.Decode(ctx, key1)
	assert.ErrorIs(t, err, sql.ErrNoRows)
}

func TestDecode_Expired(t *testing.T) {
	db, err := testDB()
	assert.NilError(t, err)

	now := time.Now()
	clock := clocks.NewFakeClock(now)

	codec := &sqliteCodec{
		db:    db,
		clock: clock,
	}

	ctx := context.Background()
	data := map[string]any{"wibble": "wobble"}

	key1, err := codec.Encode(ctx, "", data, time.Hour)
	assert.NilError(t, err)

	decoded, err := codec.Decode(ctx, key1)
	assert.NilError(t, err)
	assert.DeepEqual(t, data, decoded)

	clock.SetTime(now.Add(2 * time.Hour))

	_, err = codec.Decode(ctx, key1)
	assert.ErrorIs(t, err, sql.ErrNoRows)
}

func TestDecode_AutoExpired(t *testing.T) {
	db, err := testDB()
	assert.NilError(t, err)

	now := time.Now()
	clock := clocks.NewFakeClock(now)

	codec := &sqliteCodec{
		db:    db,
		clock: clock,
	}

	ctx := context.Background()
	data := map[string]any{"wibble": "wobble"}

	key1, err := codec.Encode(ctx, "", data, time.Hour)
	assert.NilError(t, err)

	decoded, err := codec.Decode(ctx, key1)
	assert.NilError(t, err)
	assert.DeepEqual(t, data, decoded)

	codec.expireSessions(ctx, now.Add(2*time.Hour))

	_, err = codec.Decode(ctx, key1)
	assert.ErrorIs(t, err, sql.ErrNoRows)
}
