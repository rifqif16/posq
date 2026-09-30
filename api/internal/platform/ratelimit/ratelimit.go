// Package ratelimit: limiter fixed-window in-memory untuk endpoint auth.
// Cukup untuk satu instans (V1 di VM tunggal). Pindah ke Redis saat ada >1 instans API.
package ratelimit

import (
	"net/http"
	"strconv"
	"sync"
	"time"
)

type Limiter struct {
	mu     sync.Mutex
	limit  int
	window time.Duration
	hits   map[string]*bucket
	now    func() time.Time
}

type bucket struct {
	count   int
	resetAt time.Time
}

func New(limit int, window time.Duration) *Limiter {
	return &Limiter{limit: limit, window: window, hits: make(map[string]*bucket), now: time.Now}
}

// Allow mengembalikan false bila key melewati batas; retryAfter valid saat false.
func (l *Limiter) Allow(key string) (ok bool, retryAfter time.Duration) {
	l.mu.Lock()
	defer l.mu.Unlock()
	now := l.now()
	l.evict(now)

	b, found := l.hits[key]
	if !found || now.After(b.resetAt) {
		b = &bucket{resetAt: now.Add(l.window)}
		l.hits[key] = b
	}
	if b.count >= l.limit {
		return false, b.resetAt.Sub(now)
	}
	b.count++
	return true, 0
}

// evict membuang bucket kedaluwarsa agar map tidak tumbuh tanpa batas.
func (l *Limiter) evict(now time.Time) {
	if len(l.hits) < 1024 {
		return
	}
	for k, b := range l.hits {
		if now.After(b.resetAt) {
			delete(l.hits, k)
		}
	}
}

// Middleware membatasi per key hasil keyFn. onLimit menulis respons 429.
func (l *Limiter) Middleware(keyFn func(*http.Request) string, onLimit func(http.ResponseWriter, *http.Request)) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ok, retry := l.Allow(keyFn(r))
			if !ok {
				w.Header().Set("Retry-After", strconv.Itoa(int(retry.Seconds())+1))
				onLimit(w, r)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}
