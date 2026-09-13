package redact

import (
	"crypto/sha256"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"
)

func testSessionCache(t *testing.T, change func(*SessionCacheOptions)) *SessionCache {
	t.Helper()
	options := SessionCacheOptions{TTL: time.Hour, MaxSessions: 8, MaxEntries: 100, MaxBytes: 64 * 1024}
	if change != nil {
		change(&options)
	}
	cache := NewSessionCache(options)
	if cache == nil {
		t.Fatal("invalid test cache")
	}
	t.Cleanup(cache.Close)
	return cache
}

func cacheTestKey(name string) [32]byte { return sha256.Sum256([]byte(name)) }

func cacheTestToken(t *testing.T, context *Context, raw string) string {
	t.Helper()
	token, err := context.tokenFor("SECRET", raw)
	if err != nil {
		t.Fatal(err)
	}
	return token
}

func TestSessionCacheReusesMappingsWithRequestLocalCounts(t *testing.T) {
	cache := testSessionCache(t, nil)
	key := cacheTestKey("first")
	first := cache.Context(key, 10)
	token, err := first.RedactText("alice@example.com", DetectorFlags{Email: true})
	if err != nil {
		t.Fatal(err)
	}
	second := cache.Context(key, 10)
	if second.RedactionCount() != 0 || second.RestoreCount() != 0 || len(second.RedactionFields()) != 0 {
		t.Fatal("request counters leaked through cache")
	}
	if got := second.RestoreText(token); got != "alice@example.com" || second.RestoreUniqueCount() != 1 {
		t.Fatalf("continuation not restored: %s", got)
	}
	if reused, err := second.RedactText("alice@example.com", DetectorFlags{Email: true}); err != nil || reused != token || second.RedactionCount() != 1 {
		t.Fatalf("mapping not reused: %s (%v)", reused, err)
	}
	other := cache.Context(cacheTestKey("other"), 10)
	if other.RestoreText(token) != token || other.RestoreCount() != 0 || other.UnresolvedCount() != 1 {
		t.Fatal("mapping crossed scopes")
	}
	if first.RestoreCount() != 0 || first.RedactionCount() != 1 {
		t.Fatal("later requests mutated the first context")
	}
}

func TestSessionCacheDoesNotConsumeRequestRedactionLimit(t *testing.T) {
	cache := testSessionCache(t, nil)
	key := cacheTestKey("limit")
	first := cache.Context(key, 10)
	cacheTestToken(t, first, "first")
	cacheTestToken(t, first, "second")
	current := cache.Context(key, 1)
	cacheTestToken(t, current, "new value")
	if _, err := current.tokenFor("SECRET", "first"); !errors.Is(err, ErrRedactionLimit) {
		t.Fatalf("cached values bypassed the request limit: %v", err)
	}
	current = cache.Context(key, 1)
	cacheTestToken(t, current, "first")
	if _, err := current.tokenFor("SECRET", "second"); !errors.Is(err, ErrRedactionLimit) {
		t.Fatalf("two seeded values bypassed the request limit: %v", err)
	}
}

func TestSessionCacheEvictionPreservesInflightContext(t *testing.T) {
	for _, mode := range []string{"sessions", "entries", "bytes"} {
		t.Run(mode, func(t *testing.T) {
			cache := testSessionCache(t, func(options *SessionCacheOptions) {
				switch mode {
				case "sessions":
					options.MaxSessions = 1
				case "entries":
					options.MaxEntries = 1
				case "bytes":
					options.MaxBytes = 1024
				}
			})
			key := cacheTestKey("evicted")
			active := cache.Context(key, 10)
			raw := strings.Repeat("a", 100)
			token := cacheTestToken(t, active, raw)
			other := cache.Context(cacheTestKey("newer"), 10)
			cacheTestToken(t, other, strings.Repeat("b", 100))
			cache.mu.Lock()
			_, exists := cache.sessions[key]
			bounded := cache.entries <= cache.options.MaxEntries && cache.bytes <= cache.options.MaxBytes && len(cache.sessions) <= cache.options.MaxSessions
			cache.mu.Unlock()
			if exists || !bounded || active.RestoreText(token) != raw {
				t.Fatal("eviction exceeded limits or invalidated an active response")
			}
			late := cacheTestToken(t, active, "created after eviction")
			fresh := cache.Context(key, 10)
			if fresh.HasMappings() || fresh.RestoreText(late) != late {
				t.Fatal("evicted context resurrected old mappings")
			}
		})
	}
}

func TestSessionCacheEvictsLeastRecentlyUsedSession(t *testing.T) {
	cache := testSessionCache(t, func(options *SessionCacheOptions) { options.MaxSessions = 2 })
	a, b, c := cacheTestKey("a"), cacheTestKey("b"), cacheTestKey("c")
	cacheTestToken(t, cache.Context(a, 10), "a")
	cacheTestToken(t, cache.Context(b, 10), "b")
	cache.Context(a, 10)
	cache.Context(c, 10)
	cache.mu.Lock()
	defer cache.mu.Unlock()
	if cache.sessions[a] == nil || cache.sessions[b] != nil || cache.sessions[c] == nil {
		t.Fatal("wrong session evicted")
	}
}

func TestSessionCacheOversizedMappingStaysRequestLocal(t *testing.T) {
	cache := testSessionCache(t, func(options *SessionCacheOptions) { options.MaxBytes = 1024 })
	key := cacheTestKey("large")
	active := cache.Context(key, 10)
	raw := strings.Repeat("secret", 1024)
	token := cacheTestToken(t, active, raw)
	if active.RestoreText(token) != raw {
		t.Fatal("uncached mapping cannot restore within request")
	}
	if later := cache.Context(key, 10); later.HasMappings() || later.RestoreText(token) != token {
		t.Fatal("oversized mapping was retained")
	}
	cache.mu.Lock()
	defer cache.mu.Unlock()
	if cache.entries != 0 || cache.bytes != sessionCacheOverhead {
		t.Fatalf("oversized mapping affected cache accounting: %d/%d", cache.entries, cache.bytes)
	}
}

func TestSessionCacheExpirationAndRefresh(t *testing.T) {
	cache := testSessionCache(t, nil)
	now := time.Now()
	cache.now = func() time.Time { return now }
	key := cacheTestKey("expiry")
	active := cache.Context(key, 10)
	token := cacheTestToken(t, active, "original")
	cache.mu.Lock()
	session := cache.sessions[key]
	now = now.Add(30 * time.Minute)
	cache.mu.Unlock()
	cache.Context(key, 10) // Refresh the idle deadline.
	cache.mu.Lock()
	now = now.Add(31 * time.Minute)
	cache.mu.Unlock()
	cache.expireSession(session)
	if later := cache.Context(key, 10); later.RestoreText(token) != "original" {
		t.Fatal("old timer expired a refreshed session")
	}
	cache.mu.Lock()
	now = now.Add(time.Hour)
	cache.mu.Unlock()
	cache.expireSession(session)
	if later := cache.Context(key, 10); later.HasMappings() || later.RestoreText(token) != token {
		t.Fatal("expired mapping was reused")
	}
	if active.RestoreText(token) != "original" {
		t.Fatal("TTL invalidated an in-flight context")
	}
	late := cacheTestToken(t, active, "after expiry")
	if cache.Context(key, 10).RestoreText(late) != late {
		t.Fatal("expired generation repopulated a new session")
	}
}

func TestSessionCacheExpiresWithoutFurtherRequests(t *testing.T) {
	cache := testSessionCache(t, func(options *SessionCacheOptions) { options.TTL = 20 * time.Millisecond })
	cacheTestToken(t, cache.Context(cacheTestKey("timer"), 10), "short lived")
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		cache.mu.Lock()
		empty := len(cache.sessions) == 0 && cache.entries == 0 && cache.bytes == 0
		cache.mu.Unlock()
		if empty {
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatal("idle cache retained expired mappings")
}

func TestSessionCacheConcurrentRequestsReuseToken(t *testing.T) {
	cache := testSessionCache(t, nil)
	key := cacheTestKey("concurrent")
	const workers = 32
	contexts := make([]*Context, workers)
	for index := range contexts {
		contexts[index] = cache.Context(key, 10)
	}
	results := make(chan string, workers)
	var group sync.WaitGroup
	for _, context := range contexts {
		group.Add(1)
		go func(context *Context) {
			defer group.Done()
			token, err := context.RedactText("alice@example.com", DetectorFlags{Email: true})
			if err != nil || context.RestoreText(token) != "alice@example.com" || context.RedactionCount() != 1 || context.RestoreCount() != 1 {
				results <- "invalid"
				return
			}
			results <- token
		}(context)
	}
	group.Wait()
	close(results)
	want := ""
	for token := range results {
		if want == "" {
			want = token
		}
		if token == "invalid" || token != want {
			t.Fatalf("concurrent requests issued different tokens: %s / %s", token, want)
		}
	}
	cache.Close()
	if cache.Context(key, 10).HasMappings() || contexts[0].RestoreText(want) != "alice@example.com" {
		t.Fatal("Close retained cache data or broke an active context")
	}
}
