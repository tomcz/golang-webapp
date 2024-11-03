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

func TestRoundTrip(t *testing.T) {
	db, err := testDB()
	assert.NilError(t, err)
	defer db.Close()

	now := time.Now()
	clock := clocks.NewFakeClock(now)

	store := &sqliteStore{
		db:    db,
		clock: clock,
	}

	ctx := context.Background()
	data1 := map[string]any{"wibble": "wobble"}
	data2 := map[string]any{"wibble": "waggle"}

	key1, err := store.Write(ctx, "", data1, time.Hour)
	assert.NilError(t, err)

	decoded, err := store.Read(ctx, key1)
	assert.NilError(t, err)
	assert.DeepEqual(t, data1, decoded)

	key2, err := store.Write(ctx, key1, data2, time.Hour)
	assert.NilError(t, err)
	assert.Equal(t, key1, key2)

	decoded, err = store.Read(ctx, key1)
	assert.NilError(t, err)
	assert.DeepEqual(t, data2, decoded)

	store.Delete(ctx, key1)

	_, err = store.Read(ctx, key1)
	assert.ErrorIs(t, err, sql.ErrNoRows)
}

func TestRead_Expired(t *testing.T) {
	db, err := testDB()
	assert.NilError(t, err)
	defer db.Close()

	now := time.Now()
	clock := clocks.NewFakeClock(now)

	store := &sqliteStore{
		db:    db,
		clock: clock,
	}

	ctx := context.Background()
	data := map[string]any{"wibble": "wobble"}

	key1, err := store.Write(ctx, "", data, time.Hour)
	assert.NilError(t, err)

	decoded, err := store.Read(ctx, key1)
	assert.NilError(t, err)
	assert.DeepEqual(t, data, decoded)

	clock.SetTime(now.Add(2 * time.Hour))

	_, err = store.Read(ctx, key1)
	assert.ErrorIs(t, err, sql.ErrNoRows)
}

func TestRead_AutoExpired(t *testing.T) {
	db, err := testDB()
	assert.NilError(t, err)
	defer db.Close()

	now := time.Now()
	clock := clocks.NewFakeClock(now)

	store := &sqliteStore{
		db:    db,
		clock: clock,
	}

	ctx := context.Background()
	data := map[string]any{"wibble": "wobble"}

	key1, err := store.Write(ctx, "", data, time.Hour)
	assert.NilError(t, err)

	decoded, err := store.Read(ctx, key1)
	assert.NilError(t, err)
	assert.DeepEqual(t, data, decoded)

	store.expireSessions(ctx, now.Add(2*time.Hour))

	_, err = store.Read(ctx, key1)
	assert.ErrorIs(t, err, sql.ErrNoRows)
}
