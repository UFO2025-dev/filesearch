package server

import (
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"runtime/debug"
	"strings"
	"sync"
	"time"
)

// â”€â”€ Rate Limiter (token bucket, pure Go, zero external deps) â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€

const (
	ratePerSecond = 30.0 // max sustained requests per second per IP
	rateBurst     = 15   // initial burst capacity
)

type bucket struct {
	tokens float64
	last   time.Time
	mu     sync.Mutex
}

func (b *bucket) allow() bool {
	b.mu.Lock()
	defer b.mu.Unlock()
	now := time.Now()
	b.tokens += now.Sub(b.last).Seconds() * ratePerSecond
	if b.tokens > rateBurst {
		b.tokens = rateBurst
	}
	b.last = now
	if b.tokens < 1 {
		return false
	}
	b.tokens--
	return true
}

type rateLimiterStore struct {
	mu      sync.Mutex
	buckets map[string]*bucket
}

func newRateLimiterStore() *rateLimiterStore {
	s := &rateLimiterStore{buckets: make(map[string]*bucket)}
	// Evict idle buckets every 5 minutes to avoid unbounded memory growth.
	go func() {
		t := time.NewTicker(5 * time.Minute)
		for range t.C {
			s.mu.Lock()
			now := time.Now()
			for ip, b := range s.buckets {
				b.mu.Lock()
				idle := now.Sub(b.last) > 10*time.Minute
				b.mu.Unlock()
				if idle {
					delete(s.buckets, ip)
				}
			}
			s.mu.Unlock()
		}
	}()
	return s
}

func (s *rateLimiterStore) get(ip string) *bucket {
	s.mu.Lock()
	defer s.mu.Unlock()
	if b, ok := s.buckets[ip]; ok {
		return b
	}
	b := &bucket{tokens: rateBurst, last: time.Now()}
	s.buckets[ip] = b
	return b
}

// recoveryMiddleware catches any panic in an HTTP handler, logs a stack trace,
// and returns 500 instead of crashing the whole server.
func (srv *Server) recoveryMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if rec := recover(); rec != nil {
				stack := debug.Stack()
				slog.Error("handler panic recovered",
					"panic", fmt.Sprintf("%v", rec),
					"stack", string(stack),
					"path", r.URL.Path,
				)
				writeError(w, "internal server error", http.StatusInternalServerError)
			}
		}()
		next.ServeHTTP(w, r)
	})
}

// rateLimitMiddleware rejects requests from IPs exceeding ratePerSecond req/s.
// Applied to API endpoints only (/search, /open, /health).
func (srv *Server) rateLimitMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if isAPIPath(r.URL.Path) {
			ip, _, err := net.SplitHostPort(r.RemoteAddr)
			if err != nil {
				ip = r.RemoteAddr
			}
			if !srv.limiter.get(ip).allow() {
				w.Header().Set("Retry-After", "1")
				writeError(w, "rate limit exceeded", http.StatusTooManyRequests)
				return
			}
		}
		next.ServeHTTP(w, r)
	})
}

// â”€â”€ Token Auth â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€

// authMiddleware enforces Bearer token auth on API endpoints when srv.token != "".
// Static assets (UI) are always accessible so the browser can render the page.
func (srv *Server) authMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if srv.token != "" && isAPIPath(r.URL.Path) {
			auth := r.Header.Get("Authorization")
			if auth != "Bearer "+srv.token {
				w.Header().Set("WWW-Authenticate", `Bearer realm="filesearch"`)
				writeError(w, "unauthorized", http.StatusUnauthorized)
				return
			}
		}
		next.ServeHTTP(w, r)
	})
}

// isAPIPath returns true for paths that are API endpoints (not static files).
func isAPIPath(p string) bool {
	return strings.HasPrefix(p, "/search") ||
		strings.HasPrefix(p, "/open") ||
		strings.HasPrefix(p, "/health") ||
		strings.HasPrefix(p, "/api/")
}
