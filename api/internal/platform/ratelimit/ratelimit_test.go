package ratelimit

import (
	"testing"
	"time"
)

func TestAllowBlocksAfterLimitAndResetsAfterWindow(t *testing.T) {
	l := New(2, time.Minute)
	now := time.Unix(1000, 0)
	l.now = func() time.Time { return now }

	for i := range 2 {
		if ok, _ := l.Allow("ip"); !ok {
			t.Fatalf("request %d harus diizinkan", i+1)
		}
	}
	ok, retry := l.Allow("ip")
	if ok || retry <= 0 {
		t.Fatalf("request ke-3 harus ditolak dengan retryAfter, dapat ok=%v retry=%v", ok, retry)
	}
	if ok, _ := l.Allow("other"); !ok {
		t.Fatal("key lain tidak boleh terpengaruh")
	}

	now = now.Add(time.Minute + time.Second)
	if ok, _ := l.Allow("ip"); !ok {
		t.Fatal("setelah window berlalu request harus diizinkan")
	}
}
