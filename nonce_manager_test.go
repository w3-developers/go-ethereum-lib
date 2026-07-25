package ethlib

import (
	"context"
	"errors"
	"sync"
	"testing"
)

func acquireCommit(t *testing.T, nm *NonceManager, addr string, fetch func(context.Context) (uint64, error)) uint64 {
	t.Helper()

	lease, err := nm.Acquire(context.Background(), addr, fetch)
	if err != nil {
		t.Fatal(err)
	}
	nonce := lease.Nonce()
	lease.Commit()

	return nonce
}

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

			lease, err := nm.Acquire(context.Background(), addr, fetch)
			if err != nil {
				t.Error(err)
				return
			}
			got := lease.Nonce()
			lease.Commit()

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

func TestNonceManagerLeaseSerializesSameAddress(t *testing.T) {
	nm := NewNonceManager()
	const addr = "0xserial"
	fetch := func(ctx context.Context) (uint64, error) { return 0, nil }

	first, err := nm.Acquire(context.Background(), addr, fetch)
	if err != nil {
		t.Fatal(err)
	}
	if first.Nonce() != 0 {
		t.Fatalf("expected nonce 0, got %d", first.Nonce())
	}

	secondStarted := make(chan struct{})
	secondNonce := make(chan uint64, 1)
	go func() {
		close(secondStarted)
		lease, err := nm.Acquire(context.Background(), addr, fetch)
		if err != nil {
			t.Error(err)
			return
		}
		secondNonce <- lease.Nonce()
		lease.Commit()
	}()

	<-secondStarted
	select {
	case n := <-secondNonce:
		t.Fatalf("second Acquire must block until the first lease is released, got %d", n)
	default:
	}

	first.Commit()

	if got := <-secondNonce; got != 1 {
		t.Fatalf("expected nonce 1 after commit, got %d", got)
	}
}

func TestNonceManagerRollbackReusesNonce(t *testing.T) {
	nm := NewNonceManager()
	const addr = "0xrollback"
	fetch := func(ctx context.Context) (uint64, error) { return 9, nil }

	lease, err := nm.Acquire(context.Background(), addr, fetch)
	if err != nil {
		t.Fatal(err)
	}
	if lease.Nonce() != 9 {
		t.Fatalf("expected nonce 9, got %d", lease.Nonce())
	}
	lease.Rollback()

	if got := acquireCommit(t, nm, addr, fetch); got != 9 {
		t.Fatalf("expected the rolled back nonce 9 to be reused, got %d", got)
	}
	if got := acquireCommit(t, nm, addr, fetch); got != 10 {
		t.Fatalf("expected 10 after commit, got %d", got)
	}
}

func TestNonceLeaseCommitAndRollbackAreIdempotent(t *testing.T) {
	nm := NewNonceManager()
	const addr = "0xidem"
	fetch := func(ctx context.Context) (uint64, error) { return 0, nil }

	lease, err := nm.Acquire(context.Background(), addr, fetch)
	if err != nil {
		t.Fatal(err)
	}
	lease.Commit()
	lease.Commit()
	lease.Rollback()

	if got := acquireCommit(t, nm, addr, fetch); got != 1 {
		t.Fatalf("expected 1, got %d", got)
	}
}

func TestNonceManagerSeedFailureReleasesLock(t *testing.T) {
	nm := NewNonceManager()
	const addr = "0xseedfail"

	failing := func(ctx context.Context) (uint64, error) { return 0, errors.New("rpc down") }
	if _, err := nm.Acquire(context.Background(), addr, failing); err == nil {
		t.Fatal("expected seed error")
	}

	if got := acquireCommit(t, nm, addr, func(ctx context.Context) (uint64, error) { return 3, nil }); got != 3 {
		t.Fatalf("expected 3 after successful seed, got %d", got)
	}
}

func TestNonceManagerCaseInsensitiveAndReset(t *testing.T) {
	nm := NewNonceManager()
	fetch := func(ctx context.Context) (uint64, error) { return 0, nil }

	a := acquireCommit(t, nm, "0xAbC", fetch)
	b := acquireCommit(t, nm, "0xabc", fetch)
	if a != 0 || b != 1 {
		t.Fatalf("case-insensitive counter broken: a=%d b=%d", a, b)
	}

	nm.Reset("0xABC")
	if c := acquireCommit(t, nm, "0xabc", fetch); c != 0 {
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
		if got := acquireCommit(t, nm, addr, fetch); got != want {
			t.Fatalf("expected %d, got %d", want, got)
		}
	}
	if fetchCount != 1 {
		t.Fatalf("expected exactly 1 chain fetch (seed-once), got %d", fetchCount)
	}

	nm.Reset(addr)
	if got := acquireCommit(t, nm, addr, fetch); got != 7 {
		t.Fatalf("expected re-seed to 7 after reset, got %d", got)
	}
	if fetchCount != 2 {
		t.Fatalf("expected 2 chain fetches after reset, got %d", fetchCount)
	}
}

func TestNonceManagerDoesNotFollowChainWithoutReset(t *testing.T) {
	nm := NewNonceManager()
	const addr = "0xdead"

	_ = acquireCommit(t, nm, addr, func(ctx context.Context) (uint64, error) { return 0, nil })
	_ = acquireCommit(t, nm, addr, func(ctx context.Context) (uint64, error) { return 0, nil })

	got := acquireCommit(t, nm, addr, func(ctx context.Context) (uint64, error) { return 10, nil })
	if got != 2 {
		t.Fatalf("expected local counter 2 (chain ignored until Reset), got %d", got)
	}
}

func TestNonceManagerDifferentAddressesDoNotBlock(t *testing.T) {
	nm := NewNonceManager()
	fetch := func(ctx context.Context) (uint64, error) { return 0, nil }

	held, err := nm.Acquire(context.Background(), "0xaaa", fetch)
	if err != nil {
		t.Fatal(err)
	}
	defer held.Commit()

	done := make(chan uint64, 1)
	go func() {
		lease, err := nm.Acquire(context.Background(), "0xbbb", fetch)
		if err != nil {
			t.Error(err)
			return
		}
		done <- lease.Nonce()
		lease.Commit()
	}()

	if got := <-done; got != 0 {
		t.Fatalf("expected nonce 0 for the other address, got %d", got)
	}
}
