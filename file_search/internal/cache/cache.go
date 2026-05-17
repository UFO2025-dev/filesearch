package cache

import (
	"sync"
	"time"
)

const defaultTTL = 30 * time.Second
const defaultMaxSize = 128

type entry struct {
	results []Result
	total   int
	exp     time.Time
	prev    *entry
	next    *entry
	key     string
}

// Result mirrors db.Result to avoid a circular import.
type Result struct {
	Path    string
	Snippet string
}

// Cache is a thread-safe LRU cache with TTL.
type Cache struct {
	mu      sync.Mutex
	items   map[string]*entry
	head    *entry
	tail    *entry
	size    int
	maxSize int
	ttl     time.Duration
}

// New returns a Cache with given max size and TTL.
func New(maxSize int, ttl time.Duration) *Cache {
	if maxSize <= 0 {
		maxSize = defaultMaxSize
	}
	if ttl <= 0 {
		ttl = defaultTTL
	}
	return &Cache{
		items:   make(map[string]*entry),
		maxSize: maxSize,
		ttl:     ttl,
	}
}

// Get returns cached results and true if found and not expired.
func (c *Cache) Get(key string) ([]Result, int, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	e, ok := c.items[key]
	if !ok {
		return nil, 0, false
	}
	if time.Now().After(e.exp) {
		c.remove(e)
		return nil, 0, false
	}
	c.moveToFront(e)
	return e.results, e.total, true
}

// Set stores results under key.
func (c *Cache) Set(key string, results []Result, total int) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if e, ok := c.items[key]; ok {
		e.results = results
		e.total = total
		e.exp = time.Now().Add(c.ttl)
		c.moveToFront(e)
		return
	}
	e := &entry{key: key, results: results, total: total, exp: time.Now().Add(c.ttl)}
	c.items[key] = e
	c.pushFront(e)
	c.size++
	if c.size > c.maxSize {
		c.evict()
	}
}

// Flush clears all entries (call after bulk re-indexing).
func (c *Cache) Flush() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.items = make(map[string]*entry)
	c.head = nil
	c.tail = nil
	c.size = 0
}

// InvalidateByPath removes cached entries whose results contain path.
// Use this for targeted invalidation on single-file deletion.
func (c *Cache) InvalidateByPath(path string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	for _, e := range c.items {
		for _, r := range e.results {
			if r.Path == path {
				c.remove(e)
				break
			}
		}
	}
}

func (c *Cache) pushFront(e *entry) {
	e.prev = nil
	e.next = c.head
	if c.head != nil {
		c.head.prev = e
	}
	c.head = e
	if c.tail == nil {
		c.tail = e
	}
}

func (c *Cache) moveToFront(e *entry) {
	if c.head == e {
		return
	}
	c.unlink(e)
	c.pushFront(e)
}

func (c *Cache) evict() {
	if c.tail == nil {
		return
	}
	c.remove(c.tail)
}

func (c *Cache) remove(e *entry) {
	c.unlink(e)
	delete(c.items, e.key)
	c.size--
}

func (c *Cache) unlink(e *entry) {
	if e.prev != nil {
		e.prev.next = e.next
	} else {
		c.head = e.next
	}
	if e.next != nil {
		e.next.prev = e.prev
	} else {
		c.tail = e.prev
	}
	e.prev = nil
	e.next = nil
}
