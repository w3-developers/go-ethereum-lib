package ethlib

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/ethereum/go-ethereum/core/types"
)

type rpcStub struct {
	mu           sync.Mutex
	sentNonces   []uint64
	pendingNonce uint64
	sendDelay    time.Duration
}

func (s *rpcStub) handler(t *testing.T) http.HandlerFunc {
	t.Helper()

	return func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			ID     int    `json:"id"`
			Method string `json:"method"`
			Params []any  `json:"params"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Errorf("decode rpc request: %v", err)
			return
		}

		reply := func(result any) {
			w.Header().Set("Content-Type", "application/json")
			if err := json.NewEncoder(w).Encode(map[string]any{
				"jsonrpc": "2.0",
				"id":      req.ID,
				"result":  result,
			}); err != nil {
				t.Errorf("encode rpc response: %v", err)
			}
		}

		switch req.Method {
		case "eth_gasPrice":
			reply("0x1")
		case "eth_chainId":
			reply("0x1")
		case "eth_getTransactionCount":
			s.mu.Lock()
			pending := s.pendingNonce
			s.mu.Unlock()
			reply("0x" + hex.EncodeToString([]byte{byte(pending)}))
		case "eth_sendRawTransaction":
			raw, ok := req.Params[0].(string)
			if !ok {
				t.Errorf("unexpected sendRawTransaction params: %v", req.Params)
				return
			}

			rawBytes, err := hex.DecodeString(trim0x(raw))
			if err != nil {
				t.Errorf("decode raw tx: %v", err)
				return
			}

			var tx types.Transaction
			if err := tx.UnmarshalBinary(rawBytes); err != nil {
				t.Errorf("unmarshal raw tx: %v", err)
				return
			}

			if s.sendDelay > 0 {
				time.Sleep(s.sendDelay)
			}

			s.mu.Lock()
			s.sentNonces = append(s.sentNonces, tx.Nonce())
			s.mu.Unlock()

			reply(tx.Hash().Hex())
		default:
			t.Errorf("unexpected rpc method %q", req.Method)
		}
	}
}

func TestSignAndSendTransactionBroadcastsNoncesInOrder(t *testing.T) {
	const senders = 25

	stub := &rpcStub{sendDelay: time.Millisecond}
	server := httptest.NewServer(stub.handler(t))
	defer server.Close()

	privateKey, _, err := GenerateAddress()
	if err != nil {
		t.Fatal(err)
	}

	client := New(server.URL, "", WithNonceManager(NewNonceManager()))

	var wg sync.WaitGroup
	for i := 0; i < senders; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()

			if _, err := client.SignAndSendTransaction(
				context.Background(),
				"0xabcdef",
				"0x000000000000000000000000000000000000dEaD",
				privateKey,
				21000,
				nil,
			); err != nil {
				t.Errorf("sign and send: %v", err)
			}
		}()
	}
	wg.Wait()

	stub.mu.Lock()
	sent := append([]uint64(nil), stub.sentNonces...)
	stub.mu.Unlock()

	if len(sent) != senders {
		t.Fatalf("expected %d broadcasts, got %d", senders, len(sent))
	}

	for i, nonce := range sent {
		if nonce != uint64(i) {
			t.Fatalf("broadcast %d carried nonce %d; nonces reached the node out of order: %v", i, nonce, sent)
		}
	}
}

func TestSignAndSendTransactionReturnsHashOnSendFailure(t *testing.T) {
	var callCount int
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			ID     int    `json:"id"`
			Method string `json:"method"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Errorf("decode rpc request: %v", err)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		body := map[string]any{"jsonrpc": "2.0", "id": req.ID}

		switch req.Method {
		case "eth_gasPrice", "eth_chainId":
			body["result"] = "0x1"
		case "eth_getTransactionCount":
			body["result"] = "0x0"
		case "eth_sendRawTransaction":
			callCount++
			body["error"] = map[string]any{"code": -32000, "message": "timeout"}
		default:
			t.Errorf("unexpected rpc method %q", req.Method)
		}

		if err := json.NewEncoder(w).Encode(body); err != nil {
			t.Errorf("encode rpc response: %v", err)
		}
	}))
	defer server.Close()

	privateKey, _, err := GenerateAddress()
	if err != nil {
		t.Fatal(err)
	}

	nm := NewNonceManager()
	client := New(server.URL, "", WithNonceManager(nm))

	txHash, err := client.SignAndSendTransaction(
		context.Background(),
		"0xabcdef",
		"0x000000000000000000000000000000000000dEaD",
		privateKey,
		21000,
		nil,
	)
	if err == nil {
		t.Fatal("expected send error")
	}
	if txHash == "" {
		t.Fatal("tx hash must be returned even when the broadcast fails, so the caller can reconcile")
	}
	if callCount != 1 {
		t.Fatalf("expected exactly 1 broadcast attempt, got %d", callCount)
	}

	next, err := client.SignAndSendTransaction(
		context.Background(),
		"0xabcdef",
		"0x000000000000000000000000000000000000dEaD",
		privateKey,
		21000,
		nil,
	)
	if err == nil {
		t.Fatal("expected send error")
	}
	if next == txHash {
		t.Fatal("second transaction reused the first nonce")
	}
}
