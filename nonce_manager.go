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

type NonceLease struct {
	state *nonceState
	nonce uint64
	done  bool
}

func detachedLease(nonce uint64) *NonceLease {
	return &NonceLease{nonce: nonce}
}

func (l *NonceLease) Nonce() uint64 {
	if l == nil {
		return 0
	}

	return l.nonce
}

func (l *NonceLease) Commit() {
	if l == nil || l.done {
		return
	}
	l.done = true

	if l.state == nil {
		return
	}

	l.state.next = l.nonce + 1
	l.state.mu.Unlock()
}

func (l *NonceLease) Rollback() {
	if l == nil || l.done {
		return
	}
	l.done = true

	if l.state == nil {
		return
	}

	l.state.mu.Unlock()
}

func (m *NonceManager) Acquire(
	ctx context.Context,
	address string,
	fetchPending func(context.Context) (uint64, error),
) (*NonceLease, error) {
	state := m.stateFor(normalizeNonceKey(address))

	state.mu.Lock()

	if !state.seeded {
		chainNonce, err := fetchPending(ctx)
		if err != nil {
			state.mu.Unlock()
			return nil, err
		}

		state.next = chainNonce
		state.seeded = true
	}

	return &NonceLease{state: state, nonce: state.next}, nil
}

func (m *NonceManager) Reset(address string) {
	state := m.stateFor(normalizeNonceKey(address))

	state.mu.Lock()
	defer state.mu.Unlock()

	state.next = 0
	state.seeded = false
}
