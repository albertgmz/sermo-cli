package api

import "sync"

// Cache keys. Keys with a parameter add it after a colon.
const (
	keySite          = "site"
	keyNodes         = "nodes"
	keyReactionTypes = "reactionTypes"
	keyPalette       = "palette:"
	keyPrefixes      = "prefixes:"
)

// cache keeps metadata that does not change during a session.
type cache struct {
	mu      sync.Mutex
	entries map[string]any
}

func (c *cache) get(key string) (any, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	v, ok := c.entries[key]
	return v, ok
}

func (c *cache) put(key string, v any) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.entries == nil {
		c.entries = map[string]any{}
	}
	c.entries[key] = v
}

func (c *cache) drop(key string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	delete(c.entries, key)
}

// cached returns the value for key. On a miss it calls fetch and keeps a value without an error.
// Two callers can fetch the same key at the same time. The last value stays.
func cached[T any](c *cache, key string, fetch func() (T, error)) (T, error) {
	if v, ok := c.get(key); ok {
		return v.(T), nil
	}
	v, err := fetch()
	if err != nil {
		return v, err
	}
	c.put(key, v)
	return v, nil
}
