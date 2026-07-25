package ethlib

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"

	"github.com/ethereum/go-ethereum/core/types"
)

type sendFailureStub struct {
	mu         sync.Mutex
	rpcCode    int
	rpcMessage string
	sentNonces []uint64
}

func (s *sendFailureStub) server(t *testing.T) *httptest.Server {
	t.Helper()

	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			ID     int    `json:"id"`
			Method string `json:"method"`
			Params []any  `json:"params"`
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
			raw, _ := req.Params[0].(string)
			if rawBytes, err := hex.DecodeString(trim0x(raw)); err == nil {
				var tx types.Transaction
				if err := tx.UnmarshalBinary(rawBytes); err == nil {
					s.mu.Lock()
					s.sentNonces = append(s.sentNonces, tx.Nonce())
					s.mu.Unlock()
				}
			}

			s.mu.Lock()
			code, message := s.rpcCode, s.rpcMessage
			s.mu.Unlock()

			body["error"] = map[string]any{"code": code, "message": message}
		default:
			t.Errorf("unexpected rpc method %q", req.Method)
		}

		if err := json.NewEncoder(w).Encode(body); err != nil {
			t.Errorf("encode rpc response: %v", err)
		}
	}))
}

func (s *sendFailureStub) nonces() []uint64 {
	s.mu.Lock()
	defer s.mu.Unlock()

	return append([]uint64(nil), s.sentNonces...)
}

func sendOnce(t *testing.T, client *Client, privateKey string) (string, error) {
	t.Helper()

	gasLimit := uint64(21000)

	return client.SignAndSendTransaction(
		context.Background(),
		"0xabcdef",
		"0x000000000000000000000000000000000000dEaD",
		privateKey,
		&gasLimit,
		nil,
		nil,
	)
}

func TestSignAndSendTransactionRejectedReleasesNonce(t *testing.T) {
	stub := &sendFailureStub{rpcCode: -32000, rpcMessage: "insufficient funds for gas * price + value"}
	server := stub.server(t)
	defer server.Close()

	privateKey, _, err := GenerateAddress()
	if err != nil {
		t.Fatal(err)
	}

	client := New(server.URL, "", WithNonceManager(NewNonceManager()))

	if _, err := sendOnce(t, client, privateKey); !errors.Is(err, ErrBroadcastRejected) {
		t.Fatalf("expected ErrBroadcastRejected, got %v", err)
	}
	if _, err := sendOnce(t, client, privateKey); !errors.Is(err, ErrBroadcastRejected) {
		t.Fatalf("expected ErrBroadcastRejected, got %v", err)
	}

	sent := stub.nonces()
	if len(sent) != 2 {
		t.Fatalf("expected 2 broadcast attempts, got %d", len(sent))
	}
	if sent[0] != 0 || sent[1] != 0 {
		t.Fatalf("a rejected tx must return its nonce to the pool, got %v", sent)
	}
}

func TestSignAndSendTransactionUncertainSpendsNonce(t *testing.T) {
	stub := &sendFailureStub{rpcCode: -32603, rpcMessage: "internal error"}
	server := stub.server(t)
	defer server.Close()

	privateKey, _, err := GenerateAddress()
	if err != nil {
		t.Fatal(err)
	}

	client := New(server.URL, "", WithNonceManager(NewNonceManager()))

	if _, err := sendOnce(t, client, privateKey); !errors.Is(err, ErrBroadcastUncertain) {
		t.Fatalf("expected ErrBroadcastUncertain, got %v", err)
	}
	if _, err := sendOnce(t, client, privateKey); !errors.Is(err, ErrBroadcastUncertain) {
		t.Fatalf("expected ErrBroadcastUncertain, got %v", err)
	}

	sent := stub.nonces()
	if len(sent) != 2 || sent[0] != 0 || sent[1] != 1 {
		t.Fatalf("an uncertain send must spend the nonce, got %v", sent)
	}
}

func TestSignAndSendTransactionNonceTooLowResets(t *testing.T) {
	stub := &sendFailureStub{rpcCode: -32000, rpcMessage: "nonce too low"}
	server := stub.server(t)
	defer server.Close()

	privateKey, _, err := GenerateAddress()
	if err != nil {
		t.Fatal(err)
	}

	client := New(server.URL, "", WithNonceManager(NewNonceManager()))

	if _, err := sendOnce(t, client, privateKey); !errors.Is(err, ErrNonceTooLow) {
		t.Fatalf("expected ErrNonceTooLow, got %v", err)
	}
	if _, err := sendOnce(t, client, privateKey); !errors.Is(err, ErrNonceTooLow) {
		t.Fatalf("expected ErrNonceTooLow, got %v", err)
	}

	sent := stub.nonces()
	if len(sent) != 2 || sent[0] != 0 || sent[1] != 0 {
		t.Fatalf("nonce too low must re-seed from the chain, got %v", sent)
	}
}

func TestSignAndSendTransactionAlreadyKnownKeepsNonceSpent(t *testing.T) {
	stub := &sendFailureStub{rpcCode: -32000, rpcMessage: "already known"}
	server := stub.server(t)
	defer server.Close()

	privateKey, _, err := GenerateAddress()
	if err != nil {
		t.Fatal(err)
	}

	client := New(server.URL, "", WithNonceManager(NewNonceManager()))

	_, err = sendOnce(t, client, privateKey)
	if err == nil {
		t.Fatal("expected send error")
	}
	if errors.Is(err, ErrBroadcastRejected) || errors.Is(err, ErrBroadcastUncertain) || errors.Is(err, ErrNonceTooLow) {
		t.Fatalf("already known must surface the raw rpc error, got %v", err)
	}

	if _, err := sendOnce(t, client, privateKey); err == nil {
		t.Fatal("expected send error")
	}

	sent := stub.nonces()
	if len(sent) != 2 || sent[0] != 0 || sent[1] != 1 {
		t.Fatalf("a tx already in the pool keeps its nonce spent, got %v", sent)
	}
}

func TestClassifyBroadcastError(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want broadcastOutcome
	}{
		{"nil rpc type", errors.New("dial tcp: i/o timeout"), broadcastUncertain},
		{"nonce too low", &RPCError{Code: -32000, Message: "nonce too low"}, broadcastNonceTooLow},
		{"already known", &RPCError{Code: -32000, Message: "already known"}, broadcastAccepted},
		{"replacement underpriced", &RPCError{Code: -32000, Message: "replacement transaction underpriced"}, broadcastAccepted},
		{"underpriced", &RPCError{Code: -32000, Message: "transaction underpriced"}, broadcastRejected},
		{"insufficient funds", &RPCError{Code: -32000, Message: "insufficient funds for transfer"}, broadcastRejected},
		{"intrinsic gas", &RPCError{Code: -32000, Message: "intrinsic gas too low"}, broadcastRejected},
		{"unknown rpc", &RPCError{Code: -32603, Message: "internal error"}, broadcastUncertain},
		{"wrapped", fmt.Errorf("send: %w", &RPCError{Code: -32000, Message: "invalid sender"}), broadcastRejected},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := classifyBroadcastError(tt.err); got != tt.want {
				t.Fatalf("expected outcome %d, got %d", tt.want, got)
			}
		})
	}
}
