package ethlib

import (
	"context"
	"strings"
	"sync"
)

type NonceManager struct {
	mu    sync.Mutex
	locks map[string]*sync.Mutex
	next  map[string]uint64
}

func NewNonceManager() *NonceManager {
	return &NonceManager{
		locks: make(map[string]*sync.Mutex),
		next:  make(map[string]uint64),
	}
}

func normalizeNonceKey(address string) string {
	return strings.ToLower(strings.TrimSpace(address))
}

func (m *NonceManager) lockFor(key string) *sync.Mutex {
	m.mu.Lock()
	defer m.mu.Unlock()

	lock, ok := m.locks[key]
	if !ok {
		lock = &sync.Mutex{}
		m.locks[key] = lock
	}

	return lock
}

func (m *NonceManager) Next(
	ctx context.Context,
	address string,
	fetchPending func(ctx context.Context) (uint64, error),
) (uint64, error) {
	key := normalizeNonceKey(address)

	lock := m.lockFor(key)
	lock.Lock()
	defer lock.Unlock()

	chainNonce, err := fetchPending(ctx)
	if err != nil {
		return 0, err
	}

	m.mu.Lock()
	local := m.next[key]
	nonce := local
	if chainNonce > nonce {
		nonce = chainNonce
	}
	m.next[key] = nonce + 1
	m.mu.Unlock()

	return nonce, nil
}

func (m *NonceManager) Reset(address string) {
	key := normalizeNonceKey(address)

	m.mu.Lock()
	delete(m.next, key)
	m.mu.Unlock()
}
