package thumbs

import (
	"os"
	"sync"
	"testing"
)

// TestCacheHoldsOneEntryPerKeyUnderConcurrency pins the LRU bookkeeping: two
// goroutines asking for the same thumbnail must not leave the key twice in the
// order list, which would evict a live entry early and grow the list without
// bound.
func TestCacheHoldsOneEntryPerKeyUnderConcurrency(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("LOCALAPPDATA", dir)
	t.Setenv("XDG_CACHE_HOME", dir)
	src := sampleFile(t, dir, 200, 150)
	info, err := os.Stat(src)
	if err != nil {
		t.Fatal(err)
	}

	c := New()
	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if _, gerr := c.Get(src, info.ModTime().UnixNano(), info.Size()); gerr != nil {
				t.Errorf("get: %v", gerr)
			}
		}()
	}
	wg.Wait()

	if got := c.Len(); got != 1 {
		t.Fatalf("cache holds %d entries for one file, want 1", got)
	}
	if got := c.OrderLen(); got != 1 {
		t.Fatalf("LRU order holds %d keys, want 1", got)
	}
	// Asking again must hit the cache, not append a second key.
	if _, err := c.Get(src, info.ModTime().UnixNano(), info.Size()); err != nil {
		t.Fatal(err)
	}
	if got := c.OrderLen(); got != 1 {
		t.Fatalf("after a repeat lookup the LRU order holds %d keys", got)
	}
}
