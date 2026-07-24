package ethlib

import (
	"context"
	"sync"
	"testing"
)

func TestNonceManagerNoDuplicatesUnderConcurrency(t *testing.T) {
	nm := NewNonceManager()
	const addr = "0xABCDEF0000000000000000000000000000000001"
	const n = 200

	fetch := func(ctx context.Context) (uint64, error) { return 5, nil }

	var wg sync.WaitGroup
	var mu sync.Mutex
	seen := make(map[uint64]int)

	for i := 0; i < n; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			got, err := nm.Next(context.Background(), addr, fetch)
			if err != nil {
				t.Error(err)
				return
			}
			mu.Lock()
			seen[got]++
			mu.Unlock()
		}()
	}
	wg.Wait()

	if len(seen) != n {
		t.Fatalf("expected %d distinct nonces, got %d", n, len(seen))
	}
	for nonce, c := range seen {
		if c != 1 {
			t.Fatalf("nonce %d handed out %d times", nonce, c)
		}
	}
	if _, ok := seen[5]; !ok {
		t.Fatalf("expected first nonce to be chain pending 5")
	}
	if _, ok := seen[5+n-1]; !ok {
		t.Fatalf("expected last nonce %d", 5+n-1)
	}
}

func TestNonceManagerCaseInsensitiveAndReset(t *testing.T) {
	nm := NewNonceManager()
	fetch := func(ctx context.Context) (uint64, error) { return 0, nil }

	a, _ := nm.Next(context.Background(), "0xAbC", fetch)
	b, _ := nm.Next(context.Background(), "0xabc", fetch)
	if a != 0 || b != 1 {
		t.Fatalf("case-insensitive counter broken: a=%d b=%d", a, b)
	}

	nm.Reset("0xABC")
	c, _ := nm.Next(context.Background(), "0xabc", fetch)
	if c != 0 {
		t.Fatalf("reset failed: expected 0, got %d", c)
	}
}

func TestNonceManagerSeedsOncePerAddress(t *testing.T) {
	nm := NewNonceManager()
	const addr = "0xdead"

	fetchCount := 0
	fetch := func(ctx context.Context) (uint64, error) {
		fetchCount++
		return 7, nil
	}

	for want := uint64(7); want < 7+5; want++ {
		got, err := nm.Next(context.Background(), addr, fetch)
		if err != nil {
			t.Fatal(err)
		}
		if got != want {
			t.Fatalf("expected %d, got %d", want, got)
		}
	}
	if fetchCount != 1 {
		t.Fatalf("expected exactly 1 chain fetch (seed-once), got %d", fetchCount)
	}

	nm.Reset(addr)
	got, err := nm.Next(context.Background(), addr, fetch)
	if err != nil {
		t.Fatal(err)
	}
	if got != 7 {
		t.Fatalf("expected re-seed to 7 after reset, got %d", got)
	}
	if fetchCount != 2 {
		t.Fatalf("expected 2 chain fetches after reset, got %d", fetchCount)
	}
}

func TestNonceManagerDoesNotFollowChainWithoutReset(t *testing.T) {
	nm := NewNonceManager()
	const addr = "0xdead"

	_, _ = nm.Next(context.Background(), addr, func(ctx context.Context) (uint64, error) { return 0, nil })
	_, _ = nm.Next(context.Background(), addr, func(ctx context.Context) (uint64, error) { return 0, nil })

	got, _ := nm.Next(context.Background(), addr, func(ctx context.Context) (uint64, error) { return 10, nil })
	if got != 2 {
		t.Fatalf("expected local counter 2 (chain ignored until Reset), got %d", got)
	}
}
