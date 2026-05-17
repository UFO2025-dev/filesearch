package cache

import (
	"testing"
	"time"
)

func TestGetSetBasic(t *testing.T) {
	c := New(10, time.Minute)
	c.Set("key1", []Result{{Path: "a.txt", Snippet: "hello world"}})

	results, ok := c.Get("key1")
	if !ok {
		t.Fatal("expected cache hit")
	}
	if len(results) != 1 || results[0].Path != "a.txt" {
		t.Errorf("unexpected results: %+v", results)
	}
}

func TestMiss(t *testing.T) {
	c := New(10, time.Minute)
	_, ok := c.Get("nonexistent")
	if ok {
		t.Error("expected cache miss, got hit")
	}
}

func TestTTLExpiry(t *testing.T) {
	c := New(10, 50*time.Millisecond)
	c.Set("ttl_key", []Result{{Path: "b.txt"}})

	_, ok := c.Get("ttl_key")
	if !ok {
		t.Fatal("expected hit before expiry")
	}

	time.Sleep(60 * time.Millisecond)

	_, ok = c.Get("ttl_key")
	if ok {
		t.Error("expected miss after TTL, got hit")
	}
}

func TestEviction(t *testing.T) {
	c := New(3, time.Minute) // max 3 entries
	c.Set("a", []Result{{Path: "a"}})
	c.Set("b", []Result{{Path: "b"}})
	c.Set("c", []Result{{Path: "c"}})
	// Accessing "a" makes it the most recent.
	c.Get("a")
	// Adding "d" should evict the LRU which is "b".
	c.Set("d", []Result{{Path: "d"}})

	if c.size != 3 {
		t.Errorf("expected size 3, got %d", c.size)
	}
	_, ok := c.Get("b")
	if ok {
		t.Error("expected 'b' to be evicted (LRU), but it was still cached")
	}
	_, ok = c.Get("a")
	if !ok {
		t.Error("expected 'a' to still be cached (recently accessed)")
	}
}

func TestFlush(t *testing.T) {
	c := New(10, time.Minute)
	c.Set("x", []Result{{Path: "x.txt"}})
	c.Flush()

	_, ok := c.Get("x")
	if ok {
		t.Error("expected empty cache after Flush")
	}
	if c.size != 0 {
		t.Errorf("expected size 0 after Flush, got %d", c.size)
	}
}

func TestConcurrentAccess(t *testing.T) {
	c := New(100, time.Minute)
	done := make(chan struct{})

	// Writer goroutine
	go func() {
		for i := 0; i < 1000; i++ {
			c.Set("key", []Result{{Path: "concurrent.txt"}})
		}
		close(done)
	}()

	// Reader goroutine
	for i := 0; i < 1000; i++ {
		c.Get("key")
	}
	<-done // race detector will catch any data race
}

func TestInvalidateByPath(t *testing.T) {
	c := New(10, time.Minute)
	c.Set("search:invoice", []Result{
		{Path: "/docs/invoice.pdf", Snippet: "invoice text"},
		{Path: "/docs/other.txt",  Snippet: "unrelated"},
	})
	c.Set("search:report", []Result{
		{Path: "/docs/report.txt", Snippet: "report content"},
	})

	c.InvalidateByPath("/docs/invoice.pdf")

	// Key containing the invalidated path must be gone
	_, ok := c.Get("search:invoice")
	if ok {
		t.Error("expected 'search:invoice' to be invalidated, but it was still cached")
	}
	// Unrelated key must survive
	_, ok = c.Get("search:report")
	if !ok {
		t.Error("expected 'search:report' to still be cached after unrelated invalidation")
	}
}
