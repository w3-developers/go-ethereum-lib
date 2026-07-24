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

func TestNonceManagerSelfHealsToChain(t *testing.T) {
	nm := NewNonceManager()
	const addr = "0xdead"

	for want := uint64(0); want < 3; want++ {
		got, _ := nm.Next(context.Background(), addr, func(ctx context.Context) (uint64, error) { return 0, nil })
		if got != want {
			t.Fatalf("expected %d, got %d", want, got)
		}
	}

	got, _ := nm.Next(context.Background(), addr, func(ctx context.Context) (uint64, error) { return 10, nil })
	if got != 10 {
		t.Fatalf("expected self-heal to chain nonce 10, got %d", got)
	}
}
