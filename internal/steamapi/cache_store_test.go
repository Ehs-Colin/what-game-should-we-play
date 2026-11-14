package steamapi

import (
	"context"
	"testing"
	"time"

	"github.com/Ehs-Colin/what-game-should-we-play/internal/database/dbtest"
)

const testCacheKey = "v1:appdetails:440"

func testNow() time.Time {
	return time.Date(2026, time.January, 1, 12, 0, 0, 0, time.UTC)
}

func TestPostgresCacheLoadMissing(t *testing.T) {
	db := dbtest.Open(t)
	cache := NewPostgresCache(db, nil)

	body, fresh, found, err := cache.Load(context.Background(), testCacheKey, testNow())
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if found {
		t.Error("found = true, want false")
	}
	if fresh {
		t.Error("fresh = true, want false")
	}
	if body != nil {
		t.Errorf("body = %q, want nil", body)
	}
}

func TestPostgresCacheRecordAndLoad(t *testing.T) {
	db := dbtest.Open(t)
	cache := NewPostgresCache(db, nil)
	now := testNow()
	body := []byte(`{"success":true}`)

	if err := cache.Record(context.Background(), testCacheKey, body, now); err != nil {
		t.Fatalf("Record: %v", err)
	}
	got, fresh, found, err := cache.Load(context.Background(), testCacheKey, now)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if !found {
		t.Error("found = false, want true")
	}
	if !fresh {
		t.Error("fresh = false, want true")
	}
	if string(got) != string(body) {
		t.Errorf("body = %q, want %q", got, body)
	}
}

func TestPostgresCacheLoadExpired(t *testing.T) {
	db := dbtest.Open(t)
	cache := NewPostgresCache(db, nil)
	now := testNow()
	body := []byte(`{"success":true}`)

	if err := cache.Record(context.Background(), testCacheKey, body, now); err != nil {
		t.Fatalf("Record: %v", err)
	}
	got, fresh, found, err := cache.Load(context.Background(), testCacheKey, now.Add(CacheTTL(1)))
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if !found {
		t.Error("found = false, want true")
	}
	if fresh {
		t.Error("fresh = true, want false at expires_at")
	}
	if string(got) != string(body) {
		t.Errorf("body = %q, want %q", got, body)
	}
}

func TestPostgresCacheRecordUpdatesCallCount(t *testing.T) {
	db := dbtest.Open(t)
	cache := NewPostgresCache(db, nil)
	now := testNow()
	body := []byte(`{"success":true}`)

	const records = 5
	for range records {
		if err := cache.Record(context.Background(), testCacheKey, body, now); err != nil {
			t.Fatalf("Record: %v", err)
		}
	}

	var callCount int32
	var fetchedAt, expiresAt time.Time
	if err := db.QueryRowContext(
		context.Background(),
		`SELECT call_count, fetched_at, expires_at FROM steam_api_cache WHERE cache_key = $1`,
		testCacheKey,
	).Scan(&callCount, &fetchedAt, &expiresAt); err != nil {
		t.Fatalf("query cache row: %v", err)
	}
	if callCount != records {
		t.Errorf("call_count = %d, want %d", callCount, records)
	}
	if got, want := expiresAt.Sub(fetchedAt), CacheTTL(records); got != want {
		t.Errorf("expires_at - fetched_at = %v, want %v", got, want)
	}
}

func TestPostgresCacheRecordReplacesBodyAndPreservesCreatedAt(t *testing.T) {
	db := dbtest.Open(t)
	cache := NewPostgresCache(db, nil)
	first := testNow()
	second := first.Add(time.Hour)
	firstBody := []byte(`{"version":1}`)
	secondBody := []byte(`{"version":2}`)
	ctx := context.Background()

	if err := cache.Record(ctx, testCacheKey, firstBody, first); err != nil {
		t.Fatalf("first Record: %v", err)
	}
	var createdAt time.Time
	if err := db.QueryRowContext(
		ctx,
		`SELECT created_at FROM steam_api_cache WHERE cache_key = $1`,
		testCacheKey,
	).Scan(&createdAt); err != nil {
		t.Fatalf("query created_at: %v", err)
	}

	if err := cache.Record(ctx, testCacheKey, secondBody, second); err != nil {
		t.Fatalf("second Record: %v", err)
	}
	var gotCreatedAt, updatedAt time.Time
	if err := db.QueryRowContext(
		ctx,
		`SELECT created_at, updated_at FROM steam_api_cache WHERE cache_key = $1`,
		testCacheKey,
	).Scan(&gotCreatedAt, &updatedAt); err != nil {
		t.Fatalf("query timestamps: %v", err)
	}
	if !gotCreatedAt.Equal(createdAt) {
		t.Errorf("created_at = %v, want %v", gotCreatedAt, createdAt)
	}
	if !updatedAt.Equal(second) {
		t.Errorf("updated_at = %v, want %v", updatedAt, second)
	}

	got, _, _, err := cache.Load(ctx, testCacheKey, second)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if string(got) != string(secondBody) {
		t.Errorf("body = %q, want %q", got, secondBody)
	}
}

func TestPostgresCacheRecordPrunesSlidingWindow(t *testing.T) {
	db := dbtest.Open(t)
	cache := NewPostgresCache(db, nil)
	now := testNow()
	old := now.Add(-40 * 24 * time.Hour)
	body := []byte(`{"success":true}`)
	ctx := context.Background()

	for i := range 5 {
		if err := cache.Record(ctx, testCacheKey, body, old.Add(time.Duration(i)*time.Minute)); err != nil {
			t.Fatalf("old Record: %v", err)
		}
	}
	if err := cache.Record(ctx, testCacheKey, body, now); err != nil {
		t.Fatalf("current Record: %v", err)
	}

	var callCount int32
	var fetchedAt, expiresAt time.Time
	if err := db.QueryRowContext(
		ctx,
		`SELECT call_count, fetched_at, expires_at FROM steam_api_cache WHERE cache_key = $1`,
		testCacheKey,
	).Scan(&callCount, &fetchedAt, &expiresAt); err != nil {
		t.Fatalf("query cache row: %v", err)
	}
	if callCount != 1 {
		t.Errorf("call_count = %d, want 1", callCount)
	}
	if got, want := expiresAt.Sub(fetchedAt), CacheTTL(1); got != want {
		t.Errorf("TTL = %v, want %v", got, want)
	}

	var eventCount int
	if err := db.QueryRowContext(
		ctx,
		`SELECT COUNT(*) FROM steam_api_cache_calls WHERE cache_key = $1`,
		testCacheKey,
	).Scan(&eventCount); err != nil {
		t.Fatalf("count call events: %v", err)
	}
	if eventCount != 1 {
		t.Errorf("call events = %d, want 1", eventCount)
	}
}

func TestPostgresCachePrune(t *testing.T) {
	db := dbtest.Open(t)
	cache := NewPostgresCache(db, nil)
	now := testNow()
	oldKey := "v1:appdetails:1"
	freshKey := "v1:appdetails:2"
	body := []byte(`{"success":true}`)
	ctx := context.Background()

	if err := cache.Record(ctx, oldKey, body, now.Add(-40*24*time.Hour)); err != nil {
		t.Fatalf("record old key: %v", err)
	}
	if err := cache.Record(ctx, freshKey, body, now); err != nil {
		t.Fatalf("record fresh key: %v", err)
	}

	if err := cache.Prune(ctx, now); err != nil {
		t.Fatalf("Prune: %v", err)
	}

	var oldRows int
	if err := db.QueryRowContext(
		ctx,
		`SELECT COUNT(*) FROM steam_api_cache WHERE cache_key = $1`,
		oldKey,
	).Scan(&oldRows); err != nil {
		t.Fatalf("count old cache rows: %v", err)
	}
	if oldRows != 0 {
		t.Errorf("old cache rows = %d, want 0", oldRows)
	}
	var oldEvents int
	if err := db.QueryRowContext(
		ctx,
		`SELECT COUNT(*) FROM steam_api_cache_calls WHERE cache_key = $1`,
		oldKey,
	).Scan(&oldEvents); err != nil {
		t.Fatalf("count old call events: %v", err)
	}
	if oldEvents != 0 {
		t.Errorf("old call events = %d, want 0", oldEvents)
	}

	var freshRows int
	if err := db.QueryRowContext(
		ctx,
		`SELECT COUNT(*) FROM steam_api_cache WHERE cache_key = $1`,
		freshKey,
	).Scan(&freshRows); err != nil {
		t.Fatalf("count fresh cache rows: %v", err)
	}
	if freshRows != 1 {
		t.Errorf("fresh cache rows = %d, want 1", freshRows)
	}
}

func TestPostgresCacheRecordRollsBackOnFailure(t *testing.T) {
	db := dbtest.Open(t)
	cache := NewPostgresCache(db, nil)
	ctx := context.Background()

	if _, err := db.ExecContext(ctx, `
		CREATE FUNCTION reject_cache_call() RETURNS trigger AS $$
		BEGIN
			RAISE EXCEPTION 'reject cache call';
		END;
		$$ LANGUAGE plpgsql
	`); err != nil {
		t.Fatalf("create function: %v", err)
	}
	if _, err := db.ExecContext(ctx, `
		CREATE TRIGGER reject_cache_call
		BEFORE INSERT ON steam_api_cache_calls
		FOR EACH ROW EXECUTE FUNCTION reject_cache_call()
	`); err != nil {
		t.Fatalf("create trigger: %v", err)
	}

	if err := cache.Record(ctx, testCacheKey, []byte(`{"success":true}`), testNow()); err == nil {
		t.Fatal("Record returned nil, want error")
	}

	var cacheRows int
	if err := db.QueryRowContext(
		ctx,
		`SELECT COUNT(*) FROM steam_api_cache WHERE cache_key = $1`,
		testCacheKey,
	).Scan(&cacheRows); err != nil {
		t.Fatalf("count cache rows: %v", err)
	}
	if cacheRows != 0 {
		t.Errorf("cache rows = %d, want 0", cacheRows)
	}
	var callRows int
	if err := db.QueryRowContext(
		ctx,
		`SELECT COUNT(*) FROM steam_api_cache_calls WHERE cache_key = $1`,
		testCacheKey,
	).Scan(&callRows); err != nil {
		t.Fatalf("count call events: %v", err)
	}
	if callRows != 0 {
		t.Errorf("call events = %d, want 0", callRows)
	}
}
