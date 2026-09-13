package redact

import (
	"container/list"
	"strings"
	"sync"
	"time"
)

type SessionCacheOptions struct {
	TTL         time.Duration
	MaxSessions int
	MaxEntries  int
	MaxBytes    int
}

// Accounting includes a conservative allowance for the two indexes and their
// metadata, in addition to the exact owned string lengths. Request snapshots
// are independent and live only for the duration of their requests.
const sessionCacheOverhead = 512
const mappingCacheOverhead = 256

type cachedSession struct {
	key        [32]byte
	rawToToken map[string]string
	tokenToRaw map[string]string
	expiresAt  time.Time
	bytes      int
	element    *list.Element
	timer      *time.Timer
}

type SessionCache struct {
	mu       sync.Mutex
	options  SessionCacheOptions
	sessions map[[32]byte]*cachedSession
	lru      list.List
	entries  int
	bytes    int
	closed   bool
	now      func() time.Time
}

// Invalid limits disable caching. The application validates configured limits
// at startup; this also keeps an unconfigured Proxy safely request-scoped.
func NewSessionCache(options SessionCacheOptions) *SessionCache {
	if options.TTL <= 0 || options.MaxSessions <= 0 || options.MaxEntries <= 0 || options.MaxBytes < sessionCacheOverhead {
		return nil
	}
	return &SessionCache{options: options, sessions: make(map[[32]byte]*cachedSession), now: time.Now}
}

// Context copies only this scope's mappings. Neither cache eviction nor another
// request can mutate its restoration map or its diagnostic counters.
func (c *SessionCache) Context(key [32]byte, maxRedactions int) *Context {
	context := NewContext(maxRedactions)
	if c == nil {
		return context
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.closed {
		return context
	}
	now := c.now()
	c.expireLocked(now)
	session := c.sessions[key]
	if session == nil {
		for len(c.sessions) >= c.options.MaxSessions || c.bytes > c.options.MaxBytes-sessionCacheOverhead {
			c.removeLocked(c.lru.Back().Value.(*cachedSession))
		}
		session = &cachedSession{key: key, rawToToken: make(map[string]string), tokenToRaw: make(map[string]string), bytes: sessionCacheOverhead}
		session.element = c.lru.PushFront(session)
		c.sessions[key] = session
		c.bytes += session.bytes
		session.expiresAt = now.Add(c.options.TTL)
		session.timer = time.AfterFunc(c.options.TTL, func() { c.expireSession(session) })
	} else {
		session.expiresAt = now.Add(c.options.TTL)
		session.timer.Reset(c.options.TTL)
		c.lru.MoveToFront(session.element)
	}
	for raw, token := range session.rawToToken {
		context.rawToToken[raw] = token
		context.tokenToRaw[token] = raw
	}
	context.tokenSource = func(label, raw string) (string, error) { return c.tokenFor(session, label, raw) }
	return context
}

func (c *SessionCache) tokenFor(session *cachedSession, label, raw string) (string, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.closed || c.sessions[session.key] != session || !c.now().Before(session.expiresAt) {
		if c.sessions[session.key] == session {
			c.removeLocked(session)
		}
		return newPlaceholder(label)
	}
	if token, ok := session.rawToToken[raw]; ok {
		return token, nil
	}
	var token string
	for {
		var err error
		token, err = newPlaceholder(label)
		if err != nil {
			return "", err
		}
		if _, exists := session.tokenToRaw[token]; !exists {
			break
		}
	}
	// Large values still restore within their request, without evicting the
	// entire cache for a mapping that cannot fit alongside its own session.
	remaining := c.options.MaxBytes - session.bytes - len(token) - mappingCacheOverhead
	if remaining < 0 || len(raw) > remaining {
		return token, nil
	}
	cost := len(raw) + len(token) + mappingCacheOverhead
	for c.entries >= c.options.MaxEntries || c.bytes > c.options.MaxBytes-cost {
		oldest := c.lru.Back()
		if oldest != nil && oldest.Value == session {
			oldest = oldest.Prev()
		}
		if oldest == nil {
			return token, nil
		}
		c.removeLocked(oldest.Value.(*cachedSession))
	}
	// A substring of a large request must not pin the whole request in cache.
	raw = strings.Clone(raw)
	session.rawToToken[raw] = token
	session.tokenToRaw[token] = raw
	session.bytes += cost
	c.bytes += cost
	c.entries++
	return token, nil
}

func (c *SessionCache) expireSession(session *cachedSession) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.sessions[session.key] != session {
		return
	}
	if remaining := session.expiresAt.Sub(c.now()); remaining > 0 {
		session.timer.Reset(remaining)
		return
	}
	c.removeLocked(session)
}

func (c *SessionCache) expireLocked(now time.Time) {
	for _, session := range c.sessions {
		if !now.Before(session.expiresAt) {
			c.removeLocked(session)
		}
	}
}

func (c *SessionCache) removeLocked(session *cachedSession) {
	delete(c.sessions, session.key)
	c.lru.Remove(session.element)
	c.entries -= len(session.tokenToRaw)
	c.bytes -= session.bytes
	if session.timer != nil {
		session.timer.Stop()
	}
	clear(session.rawToToken)
	clear(session.tokenToRaw)
	session.bytes = 0
}

func (c *SessionCache) Close() {
	if c == nil {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	c.closed = true
	for _, session := range c.sessions {
		c.removeLocked(session)
	}
}
