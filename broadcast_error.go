package ethlib

import (
	"errors"
	"fmt"
	"strings"
)

var (
	ErrBroadcastRejected  = errors.New("broadcast rejected")
	ErrBroadcastUncertain = errors.New("broadcast uncertain")
	ErrNonceTooLow        = errors.New("nonce too low")
)

type RPCError struct {
	Code    int
	Message string
}

func (e *RPCError) Error() string {
	return fmt.Sprintf("rpc error %d: %s", e.Code, e.Message)
}

type broadcastOutcome int8

const (
	broadcastUncertain broadcastOutcome = iota
	broadcastRejected
	broadcastAccepted
	broadcastNonceTooLow
)

var (
	acceptedBroadcastMessages = []string{
		"already known",
		"known transaction",
		"replacement transaction underpriced",
	}

	rejectedBroadcastMessages = []string{
		"insufficient funds",
		"intrinsic gas too low",
		"invalid sender",
		"exceeds block gas limit",
		"negative value",
		"oversized data",
		"transaction underpriced",
		"gas price too low",
		"max fee per gas less than block base fee",
		"gas limit reached",
	}
)

func classifyBroadcastError(err error) broadcastOutcome {
	var rpcErr *RPCError
	if !errors.As(err, &rpcErr) {
		return broadcastUncertain
	}

	message := strings.ToLower(rpcErr.Message)

	if strings.Contains(message, "nonce too low") {
		return broadcastNonceTooLow
	}

	for _, candidate := range acceptedBroadcastMessages {
		if strings.Contains(message, candidate) {
			return broadcastAccepted
		}
	}

	for _, candidate := range rejectedBroadcastMessages {
		if strings.Contains(message, candidate) {
			return broadcastRejected
		}
	}

	return broadcastUncertain
}
