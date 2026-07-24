package ethlib

import (
	"context"
	"strings"
	"sync"
)

type nonceState struct {
	mu     sync.Mutex
	next   uint64
	seeded bool
}

type NonceManager struct {
	mu     sync.Mutex
	states map[string]*nonceState
}

func NewNonceManager() *NonceManager {
	return &NonceManager{
		states: make(map[string]*nonceState),
	}
}

func normalizeNonceKey(address string) string {
	return strings.ToLower(strings.TrimSpace(address))
}

func (m *NonceManager) stateFor(key string) *nonceState {
	m.mu.Lock()
	defer m.mu.Unlock()

	state, ok := m.states[key]
	if !ok {
		state = &nonceState{}
		m.states[key] = state
	}

	return state
}

func (m *NonceManager) Next(
	ctx context.Context,
	address string,
	fetchPending func(context.Context) (uint64, error),
) (uint64, error) {
	key := normalizeNonceKey(address)
	state := m.stateFor(key)

	state.mu.Lock()
	defer state.mu.Unlock()

	if !state.seeded {
		chainNonce, err := fetchPending(ctx)
		if err != nil {
			return 0, err
		}

		state.next = chainNonce
		state.seeded = true
	}

	nonce := state.next
	state.next++

	return nonce, nil
}

func (m *NonceManager) Reset(address string) {
	key := normalizeNonceKey(address)
	state := m.stateFor(key)

	state.mu.Lock()
	defer state.mu.Unlock()

	state.next = 0
	state.seeded = false
}
